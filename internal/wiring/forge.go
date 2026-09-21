// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// forgeConnection is the forge client and the repository it serves, found once,
// on first use: resolving a token can run `gh auth token`, which is not worth
// doing for a session that never touches the forge.
type forgeConnection struct {
	client forge.Client
	repo   forge.Repo
}

// forgeDeps is what the interface asks of GitHub or GitLab.
func forgeDeps(
	ctx context.Context, settings config.Forge, where Workspace, timeout time.Duration, log *RequestLog,
) tui.ForgeDeps {
	connect := onceConnected(func() (forgeConnection, error) {
		return connectForge(ctx, settings, where.Remote, timeout, log)
	})

	return forgeDepsFrom(ctx, connect, func() []forge.Template { return templatesFor(settings, where) },
		forgeKind(settings, where.Remote))
}

// forgeDepsFrom builds the forge seams over a connect, split out so a test can
// drive their success arms with a client pointed at a fake forge rather than a
// real credential and a live GitHub.
func forgeDepsFrom(
	ctx context.Context, connect func() (forgeConnection, error), templates func() []forge.Template, kind forge.Kind,
) tui.ForgeDeps {
	return tui.ForgeDeps{
		FindPullRequest: func(branch string) (forge.PullRequest, bool, error) {
			connection, err := connect()
			if err != nil {
				return forge.PullRequest{}, false, err
			}

			return connection.client.FindPullRequest(ctx, connection.repo, branch)
		},
		CreatePullRequest: func(request forge.NewPullRequest) (forge.PullRequest, error) {
			connection, err := connect()
			if err != nil {
				return forge.PullRequest{}, err
			}

			return connection.client.CreatePullRequest(ctx, connection.repo, request)
		},
		CheckStatus: func(pull forge.PullRequest, head string) (forge.CI, error) {
			connection, err := connect()
			if err != nil {
				return forge.CI{}, err
			}

			return connection.client.CheckStatus(ctx, connection.repo, pull, head)
		},
		Rerun: func(pull forge.PullRequest, head string) (bool, error) {
			connection, err := connect()
			if err != nil {
				return false, err
			}

			return connection.client.RerunChecks(ctx, connection.repo, pull, head)
		},
		ReviewRequests: func() ([]forge.ReviewRequest, error) {
			connection, err := connect()
			if err != nil {
				return nil, err
			}

			return connection.client.ReviewRequests(ctx, connection.repo.Kind)
		},
		Templates: templates,
		Author: func() (string, error) {
			connection, err := connect()
			if err != nil {
				return "", err
			}

			identity, err := connection.client.Whoami(ctx)

			return identity.Name(), err
		},
		Kind: kind,
	}
}

// resolveRepo reads the forge repository the remote points at and applies the
// configured kind — the parse-then-kind pair the connect, the kind lookup and
// the template lookup all repeat.
func resolveRepo(settings config.Forge, remote string) (forge.Repo, error) {
	repo, err := forge.ParseRemote(remote)
	if err != nil {
		return forge.Repo{}, err
	}

	return repo.WithConfiguredKind(ForgeSettings(settings))
}

// forgeKind is which forge the remote points at, read without a network call so
// the interface can name a change correctly from the start. An unparsable remote
// is simply unknown.
func forgeKind(settings config.Forge, remote string) forge.Kind {
	repo, err := resolveRepo(settings, remote)
	if err != nil {
		return forge.KindUnknown
	}

	return repo.Kind
}

// ForgeSettings is the configuration file's say about the forge, as the forge
// package takes it. doctor reads it through here too, so the two cannot come
// to read the file differently.
func ForgeSettings(settings config.Forge) forge.Configured {
	return forge.Configured{Kind: settings.Kind, Host: settings.Host, Token: forge.Token(settings.Token.Reveal())}
}

// ForgeResolver builds the resolver that finds a forge token. The interface and
// doctor both go through it, so the two cannot come to look for a credential in
// different places.
func ForgeResolver(settings config.Forge) forge.Resolver {
	return forge.Resolver{
		Getenv: os.Getenv, Look: proc.LookPath, Run: proc.Run, Configured: ForgeSettings(settings),
	}
}

// onceConnected caches a forge connection once it succeeds, and retries after a
// failure rather than remembering it: a token added, or gh signed in, in another
// terminal is found on the next attempt instead of only on a restart.
func onceConnected(connect func() (forgeConnection, error)) func() (forgeConnection, error) {
	var (
		lock   sync.Mutex
		cached forgeConnection
		ok     bool
	)

	return func() (forgeConnection, error) {
		lock.Lock()
		defer lock.Unlock()

		if ok {
			return cached, nil
		}

		connection, err := connect()
		if err != nil {
			return forgeConnection{}, err
		}

		cached, ok = connection, true

		return cached, nil
	}
}

// connectForge finds the forge the remote points at and the token for it, the
// same way doctor --online does.
func connectForge(
	ctx context.Context, settings config.Forge, remote string, timeout time.Duration, log *RequestLog,
) (forgeConnection, error) {
	repo, err := resolveRepo(settings, remote)
	if err != nil {
		return forgeConnection{}, fmt.Errorf("reading origin: %w", err)
	}

	base, err := repo.APIBase()
	if err != nil {
		return forgeConnection{}, fmt.Errorf("%s — set forge.kind and forge.host: %w", repo.Host, err)
	}

	transport, usingCLI := forgeTransport(ctx, settings, repo, base, timeout, proc.Available)

	token, _, err := ForgeResolver(settings).Resolve(ctx, repo.Kind, repo.Host)
	if err != nil && !usingCLI {
		return forgeConnection{}, err
	}

	if token == "" {
		// The CLI transport authenticates itself; a placeholder satisfies the
		// client's token guard without a real credential to resolve.
		token = cliToken
	}

	//nolint:bodyclose // Wrap only relays the response; the forge client reads and closes its body.
	client := forge.New(log.Wrap("forge", transport), base, token)

	return forgeConnection{client: client, repo: repo}, nil
}

// templatesFor reads the repository's pull request templates, where its forge
// looks for them.
func templatesFor(settings config.Forge, where Workspace) []forge.Template {
	repo, err := resolveRepo(settings, where.Remote)
	if err != nil {
		return nil
	}

	return forge.FindTemplates(os.DirFS(where.Root), repo.Kind)
}
