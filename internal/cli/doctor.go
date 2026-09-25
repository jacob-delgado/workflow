// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Errors doctor reports. Callers distinguish them with errors.Is.
var (
	// errIncomplete reports a configuration that loaded but is missing fields.
	errIncomplete = errors.New("configuration is incomplete")
	// errInvalid reports a configuration that loaded but holds a malformed value.
	errInvalid = errors.New("configuration has an invalid value")
	// errShared reports a configuration file that is not its owner's alone.
	errShared = errors.New("the configuration file can be reached by other users")
	// errMissingTooling reports a required external program that is absent.
	errMissingTooling = errors.New("required tooling is missing")
	// errCredentialRejected reports a credential a service would not accept.
	errCredentialRejected = errors.New("a credential was rejected")
	// errUnreachable reports a service that never answered, as distinct from one
	// that answered by refusing the credential.
	errUnreachable = errors.New("the service could not be reached")
)

// labelWidth keeps the report's values in one column so the eye can scan them.
const labelWidth = 14

// newDoctorCmd builds `workflow doctor`.
func newDoctorCmd() *cobra.Command {
	var (
		online bool
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report the repository, tooling, and configuration in effect",
		Long: "Report the git repository this session is in, which external\n" +
			"programs are installed, which " + config.FileName + " is in effect,\n" +
			"and which required fields are still empty.\n\n" +
			"Makes no network calls by default, so it is safe to run anywhere and\n" +
			"tells you only that a credential is present. Add --online to ask each\n" +
			"service whether the credential actually works. Add --json for the same\n" +
			"facts as data, with the same masking.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			requestLog, closeLog, err := requestLogFor(cmd)
			if err != nil {
				return err
			}
			defer closeLog()

			cfg, loadErr := loadFromEnvironment()
			run := doctorRun{cfg: cfg, loadErr: loadErr, online: online, log: requestLog}

			if asJSON {
				return runDoctorJSON(cmd.Context(), cmd.OutOrStdout(), run)
			}

			return runDoctor(cmd.Context(), cmd.OutOrStdout(), run)
		},
	}

	cmd.Flags().BoolVar(&online, "online", false, "ask each service whether its credential works")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the report as JSON")

	return cmd
}

// doctorRun is what one doctor run reports on: the configuration and why it did
// not load, if it did not; whether to ask each service about its credential;
// and the request log those questions are outlined in, nil for none.
type doctorRun struct {
	cfg     config.Config
	loadErr error
	online  bool
	log     *wiring.RequestLog
}

// runDoctor writes the report. Every section runs even when an earlier one found
// a problem: someone running doctor wants the whole picture, not the first thing
// that went wrong.
func runDoctor(ctx context.Context, out io.Writer, run doctorRun) error {
	// The version leads the report because it is the first thing a bug report
	// needs, and doctor's output is what the bug report template invites people
	// to paste.
	field(out, "Version", buildinfo.Current())
	fmt.Fprintln(out)

	repo := reportRepository(ctx, out)
	fmt.Fprintln(out)

	toolingErr := reportTooling(out)
	fmt.Fprintln(out)

	configErr := reportConfiguration(out, run.cfg, run.loadErr)
	if run.loadErr != nil {
		return errors.Join(toolingErr, configErr)
	}

	return errors.Join(toolingErr, configErr, reportCredentials(ctx, out, run, repo.Remote))
}

// reportCredentials asks each service whether its credential works.
//
// Offline unless asked, because doctor is otherwise fast, hermetic and safe to
// run on a machine behind a proxy or on a plane — and because its output is
// what the bug report template invites people to paste.
func reportCredentials(ctx context.Context, out io.Writer, run doctorRun, remote string) error {
	if !run.online {
		fmt.Fprintf(out, "\nCredentials were not checked. Add --online to ask each service.\n")

		return nil
	}

	fmt.Fprintf(out, "\nCredentials:\n")

	doers := onlineDoers(run.cfg, run.log)

	return errors.Join(
		checkJira(ctx, out, doers.jira, run.cfg.Jira),
		checkMessaging(ctx, out, doers.messaging, messaging.APIBase, run.cfg.Messaging),
		checkForge(ctx, out, doers.forge, run.cfg.Forge, remote),
	)
}

// onlineDoer is the transport doctor's --online checks travel over: the
// redirect-refusing client, bounded by the configured request timeout so a
// hung service does not hang doctor, or the default when none is set.
func onlineDoer(cfg config.Config) httpx.Doer {
	return httpx.Client(cmp.Or(cfg.RequestTimeout(), wiring.RequestTimeout)).Do
}

// serviceDoers are the online transport once per service, each outlining its
// requests in the request log under that service's name, as the commands that
// wire the services do.
type serviceDoers struct {
	jira      httpx.Doer
	messaging httpx.Doer
	forge     httpx.Doer
}

// onlineDoers wraps the online transport for each service in log, which may be
// nil for no log.
func onlineDoers(cfg config.Config, log *wiring.RequestLog) serviceDoers {
	doer := onlineDoer(cfg)

	return serviceDoers{
		jira:      log.Wrap("jira", doer),
		messaging: log.Wrap("slack", doer),
		forge:     log.Wrap("forge", doer),
	}
}

// forgeRepo reads the remote, reporting whether there is a forge to ask about at
// all. A repository with no remote — or one whose remote names no project — is
// not a problem for doctor to fail on, which is why this answers with a bool
// rather than an error nobody acts on.
func forgeRepo(remote string) (forge.Repo, bool) {
	repo, err := forge.ParseRemote(remote)

	return repo, err == nil
}

// apiBase is the forge's API root, or false when the host names neither forge
// and the configuration does not say which it is. That is a gap in what doctor
// can check rather than a fault in the configuration, hence a bool.
func apiBase(repo forge.Repo) (string, bool) {
	base, err := repo.APIBase()

	return base, err == nil
}

// checkForge says where the forge credential comes from.
//
// It reports the SOURCE rather than the token, and does not call the forge:
// knowing which of three places a credential was taken from is what answers
// "why is it using that one?", and it costs no network round trip.
func checkForge(ctx context.Context, out io.Writer, doer forge.Doer, settings config.Forge, remote string) error {
	repo, ok := forgeRepo(remote)
	if !ok {
		fmt.Fprintf(out, "  %-10s no repository remote, so there is no forge to ask\n", "forge")

		return nil
	}

	repo, err := repo.WithConfiguredKind(wiring.ForgeSettings(settings))
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", "forge", err)

		return fmt.Errorf("%w: forge", errCredentialRejected)
	}

	base, known := apiBase(repo)
	if !known {
		fmt.Fprintf(out, "  %-10s %s is neither github.com nor gitlab.com — set forge.kind and forge.host\n",
			"forge", repo.Host)

		return nil
	}

	token, source, err := wiring.ForgeResolver(settings).Resolve(ctx, repo.Kind, repo.Host)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %s\n", "forge", noForgeTokenMessage(proc.Available, repo.Kind, repo.Host))

		return fmt.Errorf("%w: forge", errCredentialRejected)
	}

	return askForge(ctx, out, doer, base, token, source)
}

// noForgeTokenMessage explains why no forge token resolved. When gh is the
// forge's own tool and is installed, the resolver already ran it and it yielded
// nothing, so the token is missing because gh is not signed in to this host —
// which the generic list of sources cannot say, because to the resolver a
// signed-out gh looks exactly like one that is not installed at all.
func noForgeTokenMessage(available func(string) bool, kind forge.Kind, host string) string {
	if kind == forge.KindGitHub && available("gh") {
		return fmt.Sprintf("gh is installed but not signed in to %s — run `gh auth login`", host)
	}

	return "none — " + forge.Sources(kind, host)
}

// askForge asks the forge who the credential belongs to.
func askForge(
	ctx context.Context, out io.Writer, doer forge.Doer,
	base string, token forge.Token, source forge.Source,
) error {
	identity, err := forge.New(doer, base, token).Whoami(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v (token from %s)\n", "forge", err, source)

		return credentialOutcome(err, "forge")
	}

	fmt.Fprintf(out, "  %-10s authenticates as %s (token from %s)\n", "forge", identity.Name(), source)

	return nil
}

// checkMessaging asks the messaging service which workspace the bot token
// belongs to. Only a Slack bot token can be checked; a webhook is uncheckable.
func checkMessaging(
	ctx context.Context, out io.Writer, doer messaging.Doer, base string, creds config.Messaging,
) error {
	label := strings.ToLower(creds.Service())

	token, source, err := wiring.ResolveToken(ctx, creds.Token, creds.TokenCommand, creds.TokenEnv)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", label, err)

		return fmt.Errorf("%w: %s", errCredentialRejected, label)
	}

	creds.Token = token
	client := messaging.New(doer, base, creds)

	identity, err := client.AuthTest(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", label, err)

		// A webhook that cannot be checked is not a failed check. Nothing is
		// wrong with the configuration; there is simply nothing to ask, because
		// the only way to test a webhook is to post into somebody's channel.
		if errors.Is(err, messaging.ErrWebhookUncheckable) {
			return nil
		}

		return credentialOutcome(err, label)
	}

	fmt.Fprintf(out, "  %-10s %s in %s (token from %s)\n", label, identity.User, identity.Team, source)

	return nil
}

// credentialOutcome tells an unreachable service from a rejected credential, so
// the outcome names the one the reader can act on. It reads unreachable as
// every command's exit status does: a service that could not be reached, asked
// the caller to wait, or answered with a refused redirect never judged the
// credential.
func credentialOutcome(err error, service string) error {
	if (exitFamily{members: unreachableErrors()}).holds(err) {
		return fmt.Errorf("%w: %s", errUnreachable, service)
	}

	return fmt.Errorf("%w: %s", errCredentialRejected, service)
}

// checkJira asks Jira who the configured token authenticates as.
func checkJira(ctx context.Context, out io.Writer, doer jira.Doer, settings config.Jira) error {
	token, source, err := wiring.ResolveToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", "jira", err)

		return fmt.Errorf("%w: jira", errCredentialRejected)
	}

	settings.Token = token
	client := jira.New(doer, settings)

	user, err := client.Myself(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v (token from %s)\n", "jira", err, source)

		return credentialOutcome(err, "jira")
	}

	fmt.Fprintf(out, "  %-10s authenticates as %s (token from %s)\n", "jira", identify(user), source)

	return nil
}

// identify names a user, falling back to the login when an instance is
// configured to withhold display names.
func identify(user jira.User) string {
	if user.DisplayName == "" {
		return user.Name
	}

	return user.DisplayName + " (" + user.Name + ")"
}

// reportRepository describes the git repository the working directory is in. A
// directory outside any work tree is reported rather than returned as an error:
// `workflow doctor` is exactly what someone runs to find that out.
func reportRepository(ctx context.Context, out io.Writer) gitrepo.Repo {
	dir, err := os.Getwd()
	if err != nil {
		field(out, "Repository", fmt.Sprintf("(cannot read the working directory: %v)", err))

		return gitrepo.Repo{}
	}

	repo, err := gitrepo.At(proc.Run, dir).Describe(ctx)
	if err != nil {
		field(out, "Repository", noRepositoryReason(dir, err))

		return gitrepo.Repo{}
	}

	field(out, "Repository", repo.Root)
	field(out, "Branch", branchLabel(repo))
	// A remote can carry a credential just as a base URL can.
	field(out, "Remote", config.DisplayURL(repo.Remote))
	field(out, "Forge", forgeLabel(repo.Remote))

	return repo
}

// noRepositoryReason says why dir has no repository to report: git is not
// installed, or dir is outside any work tree.
func noRepositoryReason(dir string, err error) string {
	if errors.Is(err, proc.ErrNotFound) {
		return "(none — git is not on PATH)"
	}

	return fmt.Sprintf("(none — %s is not in a git work tree)", dir)
}

// forgeLabel says which forge the remote points at, and where its API lives.
func forgeLabel(remote string) string {
	if remote == "" {
		return "(no remote)"
	}

	repo, err := forge.ParseRemote(remote)
	if err != nil {
		return "(the remote does not name a repository)"
	}

	base, err := repo.APIBase()
	if err != nil {
		// A GitHub Enterprise Server and a self-managed GitLab are
		// indistinguishable from the remote alone, and their API paths differ.
		return fmt.Sprintf("%s on %s (cannot tell GitHub Enterprise from self-managed GitLab)",
			repo.Path, repo.Host)
	}

	return fmt.Sprintf("%s %s at %s", repo.Kind, repo.Path, base)
}

// branchLabel names the checked-out branch, or says why there isn't one.
func branchLabel(repo gitrepo.Repo) string {
	if repo.Detached {
		return "(detached HEAD — check out a branch before starting work)"
	}

	return repo.Branch
}

// reportConfiguration writes the configuration section. A load error is part of
// the report rather than a failure to produce one: "there is no configuration
// file" is exactly what someone running doctor is asking about.
func reportConfiguration(out io.Writer, cfg config.Config, loadErr error) error {
	if loadErr != nil {
		return reportLoadError(out, loadErr)
	}

	field(out, "Configuration", cfg.Path)
	field(out, "Jira", fmt.Sprintf("%s (%s)",
		config.DisplayURL(cfg.Jira.BaseURL), cfg.Jira.AuthMode()))
	// The target, never the credential: a webhook URL is itself the secret, and
	// this output is what the bug report template invites people to paste.
	field(out, cfg.Messaging.Service(), fmt.Sprintf("%s (%s)", cfg.Messaging.Target(), cfg.Messaging.Mode()))

	review := reviewConfiguration(cfg)
	reportSharedMode(out, cfg.Path, review)
	reportRequirements(out, cfg.Path, review)

	return review.err()
}

// reportSharedMode says when anyone but its owner can read or write the
// configuration file, and the one command that puts it right.
func reportSharedMode(out io.Writer, path string, review configReview) {
	if !review.shared {
		return
	}

	field(out, "Permissions", fmt.Sprintf("%#o, so other users can reach this file", review.mode))
	fmt.Fprintf(out, "\nIt holds credentials. Make it yours alone with `chmod 600 %s`.\n", path)
}

// reportLoadError explains a configuration that could not be read, and says what
// to do about it.
func reportLoadError(out io.Writer, loadErr error) error {
	switch {
	case errors.Is(loadErr, config.ErrNotFound):
		fmt.Fprintf(out, "%s\n", config.NoConfigHeadline)
		fmt.Fprintf(out, "\n%s\n%s\n", config.InitStep, config.DoctorStep)
		fmt.Fprintf(out, "`workflow --help` explains how to create each token.\n")
	case errors.Is(loadErr, config.ErrInvalid):
		field(out, "Configuration", "cannot be parsed")
		fmt.Fprintf(out, "\n%v\n", loadErr)
	default:
		field(out, "Configuration", "cannot be read")
		fmt.Fprintf(out, "\n%v\n", loadErr)
	}

	return loadErr
}

// field writes one aligned "Label: value" line.
func field(out io.Writer, label, value string) {
	fmt.Fprintf(out, "%-*s %s\n", labelWidth, label+":", value)
}
