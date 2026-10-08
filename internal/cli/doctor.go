// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
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
	// errCredentialRejected reports a credential refused by the service asked, as
	// distinct from a credential there was none of to ask with and from a
	// service that never answered.
	errCredentialRejected = errors.New("a credential was rejected")
	// errCredentialMissing reports a service doctor had no credential to ask
	// about: none is configured, Jira's keychain item, token_command or
	// token_env gave none, or the Slack user token was never logged in.
	errCredentialMissing = errors.New("a credential is missing")
	// errRefreshHeldBack reports a Slack user token due a refresh that a dry
	// run holds back, since the refresh writes the new pair where it is kept.
	errRefreshHeldBack = errors.New("the Slack user token is due a refresh, which --dry-run holds back; " +
		"run doctor --online without it to check the token")
	// errUnreachable reports a service that never answered, as distinct from one
	// that answered by refusing the credential.
	errUnreachable = errors.New("the service could not be reached")
	// errUnchecked reports a credential doctor could not ask about: a webhook,
	// which only posting would test, a forge it cannot name, or a Jira that is
	// not the tracker or sits at an address the client cannot use. It belongs to
	// no exit family, and credentialVerdict leaves it out of the run's verdict.
	errUnchecked = errors.New("the credential could not be checked")
)

// labelWidth keeps the report's values in one column so the eye can scan them.
const labelWidth = 14

// newDoctorCmd builds `workflow doctor`. prompt says whether stderr is a
// terminal, where --online keeps a note of the service it is asking.
func newDoctorCmd(prompt Prompt) *cobra.Command {
	var (
		online bool
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report the repository, tooling, and configuration in effect",
		Long: "Report the git repository this session is in, which external\n" +
			"programs are installed, where the store keeps its files — or why it\n" +
			"keeps none — which " + config.FileName + " is in effect, and which\n" +
			"required fields are still empty.\n\n" +
			"Makes no network calls by default, so it is safe to run anywhere and\n" +
			"tells you only that a credential is present. Add --online to ask Jira,\n" +
			"your forge and Slack whether each credential works; a webhook is left\n" +
			"unchecked. Add --json for the same facts as data, with the same masking.",
		Example: examples(
			`workflow doctor --online                      # ask each service whether its credential works`,
			`workflow --log requests.log doctor --online   # a bug report's evidence`,
		),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			requestLog, closeLog, err := requestLogFor(cmd)
			if err != nil {
				return err
			}
			defer closeLog()

			note := newProgressNote(cmd.ErrOrStderr(), prompt.IsTerminal)
			defer note.clear()

			cfg, loadErr := loadFromEnvironment(cmd)
			run := doctorRun{
				cfg: cfg, loadErr: loadErr, online: online, dryRun: dryRunRequested(cmd), log: requestLog, note: note,
				env: environmentOf(cmd),
			}
			out := note.around(outputOf(cmd)).artifact

			if asJSON {
				return runDoctorJSON(cmd.Context(), out, run)
			}

			return runDoctor(cmd.Context(), out, run)
		},
	}

	cmd.Flags().BoolVar(&online, "online", false, "ask Jira, the forge and Slack if each credential works (not a webhook)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the report as JSON")

	return cmd
}

// doctorRun is what one doctor run reports on: the configuration and why it did
// not load, if it did not; whether to check the credentials online; and the
// request log those checks are outlined in, nil for none.
type doctorRun struct {
	cfg     config.Config
	loadErr error
	online  bool
	// dryRun holds back a refresh of the Slack user token, which writes.
	dryRun bool
	log    *wiring.RequestLog
	// note names the service --online is asking while it waits for the answer.
	note *progressNote
	// env is the process doctor reports on: where it was run from, and the
	// home, variables, store and programs it finds there.
	env Environment
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

	repository, remote := repositoryFactsFor(ctx, run.env)
	reportRepository(out, repository)
	fmt.Fprintln(out)

	toolingErr := reportTooling(ctx, out, run, remote)
	fmt.Fprintln(out)

	field(out, "Store", storeFactsFor(run).label())
	fmt.Fprintln(out)

	configErr := reportConfiguration(out, run, remote)
	if run.loadErr != nil {
		return errors.Join(toolingErr, configErr)
	}

	return errors.Join(toolingErr, configErr, reportCredentials(ctx, out, run, remote))
}

// repositoryFacts is the git repository the working directory is in, or why
// there is none, as both reports give it. A directory outside any work tree is
// a fact rather than an error: `workflow doctor` is exactly what someone runs
// to find that out.
type repositoryFacts struct {
	InsideWorkTree bool   `json:"inside_work_tree"`
	Root           string `json:"root,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Detached       bool   `json:"detached"`
	Remote         string `json:"remote,omitempty"`
	Forge          string `json:"forge,omitempty"`
	Problem        string `json:"problem,omitempty"`
}

// repositoryFactsFor gathers the git facts, returning the raw remote alongside
// so the other sections can parse it — the facts carry only the masked form,
// since a remote can carry a credential just as a base URL can.
func repositoryFactsFor(ctx context.Context, env Environment) (repositoryFacts, string) {
	dir, err := env.WorkingDir()
	if err != nil {
		return repositoryFacts{Problem: fmt.Sprintf("cannot read the working directory: %v", err)}, ""
	}

	repo, err := gitrepo.At(env.Process.Run, dir).Describe(ctx)
	if err != nil {
		return repositoryFacts{Problem: noRepositoryReason(dir, err)}, ""
	}

	return repositoryFacts{
		InsideWorkTree: true, Root: repo.Root, Branch: repo.Branch, Detached: repo.Detached,
		Remote: config.DisplayURL(repo.Remote), Forge: forgeLabel(repo.Remote),
	}, repo.Remote
}

// noRepositoryReason says why dir has no repository to report: git is not
// installed, or dir is outside any work tree.
func noRepositoryReason(dir string, err error) string {
	if errors.Is(err, proc.ErrNotFound) {
		return "git is not on PATH"
	}

	return dir + " is not in a git work tree"
}

// reportRepository writes the repository section from facts, or why there is
// no repository to report.
func reportRepository(out io.Writer, facts repositoryFacts) {
	if !facts.InsideWorkTree {
		field(out, "Repository", "(none — "+facts.Problem+")")

		return
	}

	field(out, "Repository", facts.Root)
	field(out, "Branch", branchLabel(facts))
	field(out, "Remote", facts.Remote)
	field(out, "Forge", facts.Forge)
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
func branchLabel(facts repositoryFacts) string {
	if facts.Detached {
		return "(detached HEAD — check out a branch before starting work)"
	}

	return facts.Branch
}

// reportConfiguration writes the configuration section. The remote names the
// forge whose issues are the tracker when there is no Jira. A load error is part
// of the report rather than a failure to produce one: "there is no
// configuration file" is exactly what someone running doctor is asking about.
func reportConfiguration(out io.Writer, run doctorRun, remote string) error {
	if run.loadErr != nil {
		return reportLoadError(out, run.loadErr)
	}

	cfg := run.cfg

	field(out, "Configuration", cfg.Layers().String())
	field(out, "Tracker", trackerLabel(cfg, remote))
	field(out, "Jira", fmt.Sprintf("%s (%s)",
		config.DisplayURL(cfg.Jira.BaseURL), cfg.Jira.AuthMode()))
	// The target, never the credential: a webhook URL is itself the secret, and
	// this output is what the bug report template invites people to paste.
	field(out, cfg.Messaging.Service(), fmt.Sprintf("%s (%s)", cfg.Messaging.Target(), cfg.Messaging.Mode()))

	review := reviewConfiguration(cfg)
	reportSharedMode(out, review.sharedPath, review)
	reportRequirements(out, cfg.Path, review)

	return review.err()
}

// The trackers configuration.tracker names: Jira, or, with no jira.base_url,
// the forge's own issues.
const (
	trackerJira  = "jira"
	trackerForge = "forge"
)

// trackerOf names the tracker the Issues pane reads, as the JSON report carries
// it.
func trackerOf(settings config.Jira) string {
	if settings.Configured() {
		return trackerJira
	}

	return trackerForge
}

// trackerLabel says which tracker the Issues pane reads: Jira at its base URL,
// or, with no jira.base_url, the issues of the forge the remote points at.
func trackerLabel(cfg config.Config, remote string) string {
	if trackerOf(cfg.Jira) == trackerJira {
		return "Jira at " + config.DisplayURL(cfg.Jira.BaseURL)
	}

	return issuesForge(cfg.Forge, remote) + " issues (no jira.base_url)"
}

// issuesForge names the forge whose issues stand in for Jira, reading the
// remote as the Issues pane does, with forge.kind naming an on-premises host.
// A remote that names no forge leaves it unnamed.
func issuesForge(settings config.Forge, remote string) string {
	kind := wiring.ForgeKind(settings, remote)
	if kind == forge.KindUnknown {
		return "the forge's"
	}

	return kind.String()
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

// storeFacts is where the store keeps what a session can see again, as doctor
// reports it: its directory, or that the configuration turned it off, or why
// there is no directory for it, in which case it quietly keeps nothing.
type storeFacts struct {
	Dir      string `json:"dir,omitempty"`
	Disabled bool   `json:"disabled"`
	Problem  string `json:"problem,omitempty"`
}

// storeFactsFor finds the store's directory as the store itself does.
func storeFactsFor(run doctorRun) storeFacts {
	if run.cfg.Store.Disabled {
		return storeFacts{Disabled: true}
	}

	dir, err := run.env.Process.StateDir()
	if err != nil {
		return storeFacts{Problem: err.Error()}
	}

	return storeFacts{Dir: dir}
}

// label is the store's row in the prose report.
func (facts storeFacts) label() string {
	switch {
	case facts.Disabled:
		return "off (store.disabled)"
	case facts.Problem != "":
		return "off (" + facts.Problem + ")"
	default:
		return facts.Dir
	}
}

// field writes one aligned "Label: value" line.
func field(out io.Writer, label, value string) {
	fmt.Fprintf(out, "%-*s %s\n", labelWidth, label+":", value)
}
