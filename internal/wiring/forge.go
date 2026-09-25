// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"cmp"
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
// on first use, and shared by every seam that reaches the forge: resolving a
// token can run `gh auth token`, which is not worth doing for a session that
// never touches the forge, nor worth doing twice for one that does.
type forgeConnection struct {
	client forge.Client
	repo   forge.Repo
}

// forgeSetup is what connecting to the forge reads: the configuration's say, the
// workspace whose origin names the repository, how long a request may take, and
// the log that records each one.
type forgeSetup struct {
	settings config.Forge
	where    Workspace
	timeout  time.Duration
	log      *RequestLog
}

// mergeSeams builds the two merge seams over a connect, kept out of forgeDeps
// so that builder stays within its length.
func mergeSeams(ctx context.Context, connect func() (forgeConnection, error)) (
	func(forge.PullRequest, forge.MergeMethod) error,
	func() ([]forge.MergeMethod, error),
) {
	merge := func(pull forge.PullRequest, method forge.MergeMethod) error {
		connection, err := connect()
		if err != nil {
			return err
		}

		return connection.client.Merge(ctx, connection.repo, pull, method)
	}

	methods := func() ([]forge.MergeMethod, error) {
		connection, err := connect()
		if err != nil {
			return nil, err
		}

		return connection.client.MergeMethods(ctx, connection.repo)
	}

	return merge, methods
}

// forgeDeps is what the interface asks of GitHub or GitLab, each seam reaching
// the forge through connect.
func forgeDeps(ctx context.Context, setup forgeSetup, connect func() (forgeConnection, error)) tui.ForgeDeps {
	merge, mergeMethods := mergeSeams(ctx, connect)

	return tui.ForgeDeps{
		FindPullRequest: func(branch string) (forge.PullRequest, bool, error) {
			connection, err := connect()
			if err != nil {
				return forge.PullRequest{}, false, err
			}

			return connection.client.FindPullRequest(ctx, connection.repo, branch)
		},
		CreatePullRequest: createPullSeam(ctx, connect),
		EditPullRequest:   editPullSeam(ctx, connect),
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
		Merge:        merge,
		MergeMethods: mergeMethods,
		ReviewRequests: func() ([]forge.ReviewRequest, error) {
			connection, err := connect()
			if err != nil {
				return nil, err
			}

			return connection.client.ReviewRequests(ctx, connection.repo.Kind)
		},
		Templates: func() []forge.Template { return templatesFor(setup.settings, setup.where) },
		Author: func() (string, error) {
			connection, err := connect()
			if err != nil {
				return "", err
			}

			identity, err := connection.client.Whoami(ctx)

			return identity.Name(), err
		},
		Kind: ForgeKind(setup.settings, setup.where.Remote),
	}
}

// createPullSeam is the open-a-pull-request seam, split out to keep forgeDeps
// within its length: it connects, then asks the client to open it.
func createPullSeam(
	ctx context.Context, connect func() (forgeConnection, error),
) func(forge.NewPullRequest) (forge.PullRequest, error) {
	return func(request forge.NewPullRequest) (forge.PullRequest, error) {
		connection, err := connect()
		if err != nil {
			return forge.PullRequest{}, err
		}

		return connection.client.CreatePullRequest(ctx, connection.repo, request)
	}
}

// editPullSeam is the edit-pull-request seam, split out to keep forgeDeps within
// its length: it connects, then asks the client to edit the title and body.
func editPullSeam(
	ctx context.Context, connect func() (forgeConnection, error),
) func(forge.PullRequest, forge.PullRequestEdit) (forge.PullRequest, error) {
	return func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
		connection, err := connect()
		if err != nil {
			return forge.PullRequest{}, err
		}

		return connection.client.EditPullRequest(ctx, connection.repo, pull, edit)
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

// ForgeKind is which forge the remote points at, read without a network call so
// the interface can name a change correctly from the start. An unparsable remote
// is simply unknown. doctor names the tracker's forge through it too, so the two
// cannot come to read the remote differently.
func ForgeKind(settings config.Forge, remote string) forge.Kind {
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

// connectForge finds the forge the workspace's origin points at and the token
// for it, the same way doctor --online does.
func connectForge(ctx context.Context, setup forgeSetup) (forgeConnection, error) {
	repo, err := resolveRepo(setup.settings, setup.where.Remote)
	if err != nil {
		return forgeConnection{}, fmt.Errorf("reading origin: %w", err)
	}

	base, err := repo.APIBase()
	if err != nil {
		return forgeConnection{}, fmt.Errorf("%s — set forge.kind and forge.host: %w", repo.Host, err)
	}

	access, err := ReachForge(ctx, setup.settings, repo, base, setup.timeout)
	if err != nil {
		return forgeConnection{}, err
	}

	//nolint:bodyclose // Wrap only relays the response; the forge client reads and closes its body.
	client := forge.New(setup.log.Wrap("forge", access.Doer), base, access.Token)

	return forgeConnection{client: client, repo: repo}, nil
}

// ForgeAccess is how requests reach a forge: the transport they travel over,
// the token the client carries, and where the credential came from.
type ForgeAccess struct {
	// Doer carries each request: the forge's own CLI when forge.cli routes it
	// there, otherwise the redirect-refusing HTTP client.
	Doer forge.Doer
	// Token is the resolved credential, or a placeholder the CLI never reads.
	Token forge.Token
	// Via names the credential's source for a report — "through gh", or "token
	// from the environment" — and never the credential itself.
	Via string
}

// ReachForge chooses how requests reach repo's forge at base, and finds the
// credential they carry. A forge reached through its CLI needs no token, since
// the CLI signs every request with the login it already holds. The commands and
// doctor --online both reach the forge through here, so the two cannot come to
// reach it differently.
func ReachForge(
	ctx context.Context, settings config.Forge, repo forge.Repo, base string, timeout time.Duration,
) (ForgeAccess, error) {
	transport, usingCLI := forgeTransport(ctx, settings, repo, base, timeout)

	token, source, err := ForgeResolver(settings).Resolve(ctx, repo.Kind, repo.Host)
	if !usingCLI {
		if err != nil {
			return ForgeAccess{}, err
		}

		return ForgeAccess{Doer: transport, Token: token, Via: "token from " + source.String()}, nil
	}

	program, _ := forgeProgram(repo.Kind)

	// The CLI transport authenticates itself; a placeholder satisfies the
	// client's token guard without a real credential to resolve.
	return ForgeAccess{Doer: transport, Token: cmp.Or(token, cliToken), Via: "through " + program}, nil
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
