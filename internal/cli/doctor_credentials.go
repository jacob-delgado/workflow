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
