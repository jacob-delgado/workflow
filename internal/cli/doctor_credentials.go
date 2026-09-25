// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

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

	return credentialVerdict(
		checkJira(ctx, out, doers.jira, run.cfg.Jira),
		checkMessaging(ctx, out, doers.messaging, messaging.APIBase, run.cfg.Messaging),
		checkForge(ctx, out, run, remote),
	)
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
	return cmp.Or(cfg.RequestTimeout(), wiring.RequestTimeout)
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
func checkForge(ctx context.Context, out io.Writer, run doctorRun, remote string) error {
	repo, ok := forgeRepo(remote)
	if !ok {
		return credentialUnchecked(out, "forge", "no repository remote, so there is no forge to ask")
	}

	// The configuration section fails a forge.kind that cannot be used; here it
	// only means there is no forge to ask.
	repo, err := repo.WithConfiguredKind(wiring.ForgeSettings(run.cfg.Forge))
	if err != nil {
		return credentialUnchecked(out, "forge", err.Error())
	}

	base, known := apiBase(repo)
	if !known {
		return credentialUnchecked(out, "forge",
			repo.Host+" is neither github.com nor gitlab.com — set forge.kind and forge.host")
	}

	// One limit for the whole check: a request through gh or glab runs under this
	// context, which the HTTP client's own timeout does not reach.
	timeout := onlineTimeout(run.cfg)
	gaveUp := fmt.Errorf("%w: gave up after %s", forge.ErrUnreachable, timeout)

	ctx, cancel := context.WithTimeoutCause(ctx, timeout, gaveUp)
	defer cancel()

	access, err := wiring.ReachForge(ctx, run.cfg.Forge, repo, base, httpx.Client(timeout).Do)
	if err != nil {
		return credentialMissing(out, "forge", noForgeTokenMessage(proc.Available, repo.Kind, repo.Host))
	}

	client := forge.New(run.log.Wrap("forge", access.Doer), base, access.Token)

	return askForge(ctx, out, client, access.Via)
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

// askForge asks the forge who the credential belongs to, and says beside the
// answer where that credential came from, or which CLI signed the request.
func askForge(ctx context.Context, out io.Writer, client forge.Client, via string) error {
	identity, err := client.Whoami(ctx)
	if err != nil {
		fmt.Fprintf(out, "  %-10s %v (%s)\n", "forge", unansweredBecause(ctx, err), via)

		return credentialOutcome(err, "forge")
	}

	fmt.Fprintf(out, "  %-10s authenticates as %s (%s)\n", "forge", identity.Name(), via)

	return nil
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

// checkMessaging asks the messaging service which workspace the bot token
// belongs to. Only a Slack bot token can be checked; a webhook is uncheckable.
func checkMessaging(
	ctx context.Context, out io.Writer, doer messaging.Doer, base string, creds config.Messaging,
) error {
	label := strings.ToLower(creds.Service())

	token, source, err := wiring.ResolveToken(ctx, creds.Token, creds.TokenCommand, creds.TokenEnv)
	if err != nil {
		return credentialMissing(out, label, err.Error())
	}

	if token == "" && creds.Mode() == config.MessagingBot {
		return credentialMissing(out, label, "no token from "+source)
	}

	creds.Token = token
	client := messaging.New(doer, base, creds)

	identity, err := client.AuthTest(ctx)
	// A webhook that cannot be checked is not a failed check. Nothing is wrong
	// with the configuration; there is simply nothing to ask, because the only
	// way to test a webhook is to post into somebody's channel.
	if errors.Is(err, messaging.ErrWebhookUncheckable) {
		return credentialUnchecked(out, label, err.Error())
	}

	if err != nil {
		fmt.Fprintf(out, "  %-10s %v\n", label, err)

		return credentialOutcome(err, label)
	}

	fmt.Fprintf(out, "  %-10s %s in %s (token from %s)\n", label, identity.User, identity.Team, source)

	return nil
}

// credentialOutcome names what a failed check found, so the outcome is the one
// the reader can act on: no credential to ask with, a service that never judged
// the one it was given, or a credential refused, by the service or by the
// client before sending it. It reads unreachable as every command's exit status
// does: a service not reached, one asking to wait, or a refused redirect.
func credentialOutcome(err error, service string) error {
	noCredential := exitFamily{members: []error{jira.ErrNoCredential, messaging.ErrNoCredential, forge.ErrNoToken}}

	switch {
	case noCredential.holds(err):
		return fmt.Errorf("%w: %s", errCredentialMissing, service)
	case (exitFamily{members: unreachableErrors()}).holds(err):
		return fmt.Errorf("%w: %s", errUnreachable, service)
	default:
		return fmt.Errorf("%w: %s", errCredentialRejected, service)
	}
}

// credentialMissing says why service has no credential to ask about, and
// reports it missing rather than rejected: nothing was put to the service.
func credentialMissing(out io.Writer, service, why string) error {
	fmt.Fprintf(out, "  %-10s %s\n", service, why)

	return fmt.Errorf("%w: %s", errCredentialMissing, service)
}

// credentialUnchecked says why doctor could not ask about service's credential,
// and reports it unchecked: a check not made is neither a pass nor a failure.
func credentialUnchecked(out io.Writer, service, why string) error {
	fmt.Fprintf(out, "  %-10s %s\n", service, why)

	return fmt.Errorf("%w: %s", errUnchecked, service)
}

// checkJira asks Jira who the configured token authenticates as. With no
// jira.base_url there is no Jira to ask: the forge's issues are the tracker, and
// the forge's own check covers them.
func checkJira(ctx context.Context, out io.Writer, doer jira.Doer, settings config.Jira) error {
	if !settings.Configured() {
		return credentialUnchecked(out, "jira", "not configured — the forge's issues are the tracker")
	}

	token, source, err := wiring.ResolveToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
	if err != nil {
		return credentialMissing(out, "jira", err.Error())
	}

	if token == "" && settings.AuthMode() != config.AuthNone {
		return credentialMissing(out, "jira", "no token from "+source)
	}

	settings.Token = token
	client := jira.New(doer, settings)

	user, err := client.Myself(ctx)
	// The configuration section fails an address the client cannot use; here it
	// only means there is no Jira to ask.
	if errors.Is(err, jira.ErrInvalidBaseURL) {
		return credentialUnchecked(out, "jira", err.Error()+" — the configuration section fails it; Jira was not asked")
	}

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
