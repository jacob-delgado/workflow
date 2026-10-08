// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// reportCredentials puts each credential that can be checked to its service.
//
// Offline unless asked, because doctor is otherwise fast, hermetic and safe to
// run on a machine behind a proxy or on a plane — and because its output is
// what the bug report template invites people to paste.
func reportCredentials(ctx context.Context, out io.Writer, run doctorRun, remote string) error {
	if !run.online {
		fmt.Fprint(out, "\nCredentials were not checked. Add --online to ask Jira, your forge and Slack; "+
			"a webhook is left unchecked.\n")

		return nil
	}

	fmt.Fprint(out, "\nCredentials:\n")

	return checkCredentials(ctx, run, remote, func(line credentialLine) {
		fmt.Fprintf(out, "  %-10s %s\n", line.Service, line.Detail)
	})
}

// credentialLine is one service's check, as both reports give it: the
// service, how the check went, and the already-masked detail.
type credentialLine struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
}

// checkCredentials makes each online check in turn, naming the service it is
// asking in the progress note, hands answered each one's line as it comes,
// and returns the run's verdict.
func checkCredentials(ctx context.Context, run doctorRun, remote string, answered func(credentialLine)) error {
	checks := credentialChecks(ctx, run, remote)
	outcomes := make([]error, 0, len(checks))

	for _, check := range checks {
		run.note.show("Checking", check.name)

		line, err := check.run()
		line.Status = credentialStatus(err)
		answered(line)

		outcomes = append(outcomes, err)
	}

	return credentialVerdict(outcomes...)
}

// credentialStatus names an outcome for the reader to act on: a working
// credential, one doctor could not ask about, none to ask with, one the service
// refused, or a service that never answered.
func credentialStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errUnchecked):
		return "unchecked"
	case errors.Is(err, errCredentialMissing):
		return "missing"
	case errors.Is(err, errUnreachable):
		return "unreachable"
	default:
		return "rejected"
	}
}

// credentialCheck is one service doctor --online asks: its name in the
// progress note, and the check, which answers with its line and its outcome.
type credentialCheck struct {
	name string
	run  func() (credentialLine, error)
}

// credentialChecks are the online checks, in the order both reports make
// them: Jira, the messaging service, then the forge the remote names.
func credentialChecks(ctx context.Context, run doctorRun, remote string) []credentialCheck {
	cfg, doers := run.cfg, onlineDoers(run.cfg, run.log)

	return []credentialCheck{
		{name: "Jira", run: func() (credentialLine, error) { return checkJira(ctx, doers.jira, cfg.Jira) }},
		{
			name: cfg.Messaging.Service(),
			run:  func() (credentialLine, error) { return checkMessaging(ctx, doers.messaging, run) },
		},
		{
			name: forgeNoun(wiring.ForgeKind(cfg.Forge, remote)),
			run:  func() (credentialLine, error) { return checkForge(ctx, run, remote) },
		},
	}
}

// forgeNoun names a forge mid-sentence: by its own name, or as the forge when
// the remote does not say which.
func forgeNoun(kind forge.Kind) string {
	if kind == forge.KindUnknown {
		return "the forge"
	}

	return kind.String()
}

// credentialVerdict joins the checks' outcomes into the run's verdict, leaving
// out a check doctor could not make: nothing is known to be wrong with a
// credential nobody could ask about, so it must not fail the run.
func credentialVerdict(outcomes ...error) error {
	var counted []error

	for _, outcome := range outcomes {
		if !errors.Is(outcome, errUnchecked) {
			counted = append(counted, outcome)
		}
	}

	return errors.Join(counted...)
}

// onlineTimeout bounds each of doctor's --online checks by the configured
// request timeout, so a hung service does not hang doctor, or by the default
// when none is set.
func onlineTimeout(cfg config.Config) time.Duration {
	return cmp.Or(cfg.RequestTimeout(), config.DefaultRequestTimeout)
}

// onlineDoer is the transport the tracker and messaging checks travel over: the
// redirect-refusing client, bounded by onlineTimeout.
func onlineDoer(cfg config.Config) httpx.Doer {
	return httpx.Client(onlineTimeout(cfg)).Do
}

// serviceDoers are the online transport once per HTTP-only service, each
// outlining its requests in the request log under that service's name, as the
// commands that wire the services do. The forge is not among them: forge.cli
// can route it through gh or glab, which wiring.ReachForge decides.
type serviceDoers struct {
	jira      httpx.Doer
	messaging httpx.Doer
}

// onlineDoers wraps the online transport for each service in log, which may be
// nil for no log.
func onlineDoers(cfg config.Config, log *wiring.RequestLog) serviceDoers {
	doer := onlineDoer(cfg)

	return serviceDoers{
		jira:      log.Wrap("jira", doer),
		messaging: log.Wrap("slack", doer),
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

// checkForge asks the forge who the credential belongs to, reaching it the way
// the commands do — through gh or glab when forge.cli routes it there — and
// says how it was reached.
//
// It reports the credential's SOURCE rather than the token: knowing which of
// three places a credential was taken from, or that the forge's own CLI signed
// the request, is what answers "why is it using that one?".
func checkForge(ctx context.Context, run doctorRun, remote string) (credentialLine, error) {
	repo, ok := forgeRepo(remote)
	if !ok {
		return credentialUnchecked("forge", "no repository remote, so there is no forge to ask")
	}

	// The configuration section fails a forge.kind that cannot be used; here it
	// only means there is no forge to ask.
	repo, err := repo.WithConfiguredKind(wiring.ForgeSettings(run.cfg.Forge))
	if err != nil {
		return credentialUnchecked("forge", err.Error())
	}

	base, known := apiBase(repo)
	if !known {
		return credentialUnchecked("forge",
			repo.Host+" is not github.com, a ghe.com tenant or gitlab.com — set forge.kind and forge.host")
	}

	// One limit for the whole check: a request through gh or glab runs under this
	// context, which the HTTP client's own timeout does not reach.
	timeout := onlineTimeout(run.cfg)
	gaveUp := fmt.Errorf("%w: gave up after %s", forge.ErrUnreachable, timeout)

	ctx, cancel := context.WithTimeoutCause(ctx, timeout, gaveUp)
	defer cancel()

	access, err := wiring.ReachForge(ctx, run.cfg.Forge, repo, base, httpx.Client(timeout).Do)
	if err != nil {
		return credentialMissing("forge", noForgeTokenMessage(proc.Available, repo.Kind, repo.Host))
	}

	client := forge.New(run.log.Wrap("forge", access.Doer), base, access.Token).On(repo.Kind)

	return askForge(ctx, client, repo.Kind, access.Via)
}

// noForgeTokenMessage explains why no forge token resolved. When gh is the
// forge's own tool and is installed, the resolver already ran it and it yielded
// nothing, so the token is missing because gh is not signed in to this host —
// which the generic list of sources cannot say, because to the resolver a
// signed-out gh looks exactly like one that is not installed at all. An
// installed glab is never asked for its token, so someone signed in with it
// is told where the token is read from instead, and that forge.cli goes
// through glab's own login.
func noForgeTokenMessage(available func(string) bool, kind forge.Kind, host string) string {
	switch {
	case kind == forge.KindGitHub && available("gh"):
		return fmt.Sprintf("gh is installed but not signed in to %s; run `gh auth login`", host)
	case kind == forge.KindGitLab && available("glab"):
		return "none — glab is installed, but its login is not read: " + forge.Sources(kind, host) +
			", or turn on forge.cli to go through glab"
	default:
		return "none — " + forge.Sources(kind, host)
	}
}

// askForge asks the forge who the credential belongs to, and says beside the
// answer where that credential came from, or which CLI signed the request —
// and, on GitLab, when the token can read but not write.
func askForge(ctx context.Context, client forge.Client, kind forge.Kind, via string) (credentialLine, error) {
	identity, err := client.Whoami(ctx)
	if err != nil {
		return credentialFailed("forge", fmt.Sprintf("%s (%s)", unansweredBecause(ctx, err), via), err)
	}

	return credentialLine{
		Service: "forge",
		Detail:  fmt.Sprintf("authenticates as %s (%s)%s", identity.Name(), via, writeScopeNote(ctx, client, kind)),
	}, nil
}

// writeScopeNote warns of a GitLab token without the api scope, which reads
// every issue and merge request but has every write refused. It says nothing
// where the scopes cannot be read — GitHub, an OAuth token, an older GitLab —
// and does not fail the check: the credential works, only not for writing.
func writeScopeNote(ctx context.Context, client forge.Client, kind forge.Kind) string {
	if kind != forge.KindGitLab {
		return ""
	}

	scopes, err := client.TokenScopes(ctx)
	if err != nil || len(scopes) == 0 || slices.Contains(scopes, "api") {
		return ""
	}

	if slices.Contains(scopes, "read_api") {
		return "; can read but not write: it has read_api, not api"
	}

	return "; cannot use the API: it has " + strings.Join(scopes, ", ") + ", not api"
}

// unansweredBecause says why a forge request failed: its own error, or why the
// check's context ended when that is what stopped it, since a gh or glab killed
// at the time limit can say only that it was killed.
func unansweredBecause(ctx context.Context, err error) string {
	if ctx.Err() != nil {
		return context.Cause(ctx).Error()
	}

	return err.Error()
}

// checkMessaging asks the messaging service which workspace the user token
// belongs to, refreshing the token first when it is about to run out, as a
// post would. Only a Slack user token can be checked; a webhook is
// uncheckable.
func checkMessaging(ctx context.Context, doer messaging.Doer, run doctorRun) (credentialLine, error) {
	cfg := run.cfg
	label := strings.ToLower(cfg.Messaging.Service())

	base, toSlack := wiring.SlackAPI(doer)

	client := messaging.New(toSlack, base, cfg.Messaging)
	if cfg.Messaging.Mode() == config.MessagingUser {
		client = client.WithToken(userTokenSource(cfg, doer, run.dryRun))
	}

	identity, err := client.AuthTest(ctx)
	// A webhook that cannot be checked is not a failed check. Nothing is wrong
	// with the configuration; there is simply nothing to ask, because the only
	// way to test a webhook is to post into somebody's channel. A token due a
	// refresh under --dry-run is the same: asking would mean writing.
	if errors.Is(err, messaging.ErrWebhookUncheckable) || errors.Is(err, errRefreshHeldBack) {
		return credentialUnchecked(label, err.Error())
	}

	if err != nil {
		return credentialFailed(label, err.Error(), err)
	}

	return credentialLine{
		Service: label, Detail: fmt.Sprintf("%s in %s (%s)", identity.User, identity.Team, userTokenNote(ctx, cfg)),
	}, nil
}

// userTokenSource is the user token doctor asks Slack about: the one a post
// would use, or under a dry run the one held, without the refresh that would
// write a new one where it is kept.
func userTokenSource(cfg config.Config, doer messaging.Doer, dryRun bool) messaging.TokenSource {
	if !dryRun {
		return wiring.SlackToken(cfg, doer)
	}

	store := wiring.SlackStore(cfg)

	return func(ctx context.Context, _ config.Secret) (config.Secret, error) {
		held, err := store.Load(ctx)
		if err != nil {
			return "", fmt.Errorf("%w: %w", messaging.ErrNoCredential, err)
		}

		if held.AccessToken == "" || !time.Now().Before(held.ExpiresAt) {
			return "", errRefreshHeldBack
		}

		return held.AccessToken, nil
	}
}

// userTokenNote says where cfg keeps its user token and how long it has left,
// so the reader knows which keychain or file to look in and when the next
// refresh is due.
func userTokenNote(ctx context.Context, cfg config.Config) string {
	store := wiring.SlackStore(cfg)

	held, err := store.Load(ctx)
	if err != nil {
		return "user token, kept in " + store.Where()
	}

	left := time.Until(held.ExpiresAt).Round(time.Minute)

	return fmt.Sprintf("user token, kept in %s, expires in %s", store.Where(), left)
}

// credentialOutcome names what a failed check found, so the outcome is the one
// the reader can act on: no credential to ask with, a service that never judged
// the one it was given, or a credential the service refused. It reads
// unreachable as every command's exit status does: a service not reached, one
// asking to wait, or a refused redirect.
func credentialOutcome(err error, service string) error {
	switch {
	case isAny(err, jira.ErrNoCredential, messaging.ErrNoCredential, forge.ErrNoToken):
		return fmt.Errorf("%w: %s", errCredentialMissing, service)
	case isAny(err, unreachableErrors()...):
		return fmt.Errorf("%w: %s", errUnreachable, service)
	default:
		return fmt.Errorf("%w: %s", errCredentialRejected, service)
	}
}

// credentialFailed is service's line when asking it failed with err, said as
// detail, and the outcome credentialOutcome finds in err.
func credentialFailed(service, detail string, err error) (credentialLine, error) {
	return credentialLine{Service: service, Detail: detail}, credentialOutcome(err, service)
}

// credentialMissing says why service has no credential to ask about, and
// reports it missing rather than rejected: nothing was put to the service.
func credentialMissing(service, why string) (credentialLine, error) {
	return credentialLine{Service: service, Detail: why}, fmt.Errorf("%w: %s", errCredentialMissing, service)
}

// credentialUnchecked says why doctor could not ask about service's credential,
// and reports it unchecked: a check not made is neither a pass nor a failure.
func credentialUnchecked(service, why string) (credentialLine, error) {
	return credentialLine{Service: service, Detail: why}, fmt.Errorf("%w: %s", errUnchecked, service)
}

// checkJira asks Jira who the configured token authenticates as. With no
// jira.base_url there is no Jira to ask: the forge's issues are the tracker, and
// the forge's own check covers them.
func checkJira(ctx context.Context, doer jira.Doer, settings config.Jira) (credentialLine, error) {
	if !settings.Configured() {
		return credentialUnchecked("jira", "not configured — the forge's issues are the tracker")
	}

	token, source, err := wiring.ResolveToken(ctx, settings, wiring.SystemKeychain())
	if err != nil {
		return credentialMissing("jira", err.Error())
	}

	if token == "" && settings.AuthMode() != config.AuthNone {
		return credentialMissing("jira", "no token from "+source)
	}

	settings.Token = token
	client := jira.New(doer, settings)

	user, err := client.Myself(ctx)
	// The configuration section fails an address the client cannot use, whether
	// it is not an https URL or an http one to this machine, or it carries a
	// login; here it only means
	// there is no Jira to ask.
	if errors.Is(err, config.ErrInvalidBaseURL) || errors.Is(err, config.ErrCredentialInBaseURL) {
		return credentialUnchecked("jira", err.Error()+" — the configuration section fails it; Jira was not asked")
	}

	if err != nil {
		return credentialFailed("jira", fmt.Sprintf("%v (token from %s)", err, source), err)
	}

	return credentialLine{
		Service: "jira", Detail: fmt.Sprintf("authenticates as %s (token from %s)", setup.Identify(user), source),
	}, nil
}
