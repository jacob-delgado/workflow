// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
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
// workspace whose origin names the repository, the HTTP transport every service
// shares, and the log that records each request.
type forgeSetup struct {
	// settings are the forge settings in effect now, which the web's Settings
	// may have replaced since workflow started.
	settings      func() config.Forge
	where         Workspace
	httpTransport httpx.Doer
	log           *RequestLog
}

// forgeDeps is what a surface asks of GitHub or GitLab, each seam reaching
// the forge through connect.
func forgeDeps(ctx context.Context, setup forgeSetup, connect func() (forgeConnection, error)) seams.Forge {
	kind := ForgeKind(setup.settings(), setup.where.Remote)

	return seams.Forge{
		FindPullRequest: func(branch string) (forge.PullRequest, bool, error) {
			connection, err := connect()
			if err != nil {
				return forge.PullRequest{}, false, err
			}

			return connection.client.FindPullRequest(ctx, connection.repo, branch)
		},
		CreatePullRequest: func(request forge.NewPullRequest) (forge.PullRequest, error) {
			return ask(connect, func(on forgeConnection) (forge.PullRequest, error) {
				return on.client.CreatePullRequest(ctx, on.repo, request)
			})
		},
		Activity: func(start, end time.Time) (forge.Activity, error) {
			return ask(connect, func(on forgeConnection) (forge.Activity, error) {
				return on.client.Activity(ctx, on.repo.Kind, start, end)
			})
		},
		EditPullRequest: func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error) {
			return ask(connect, func(on forgeConnection) (forge.PullRequest, error) {
				return on.client.EditPullRequest(ctx, on.repo, pull, edit)
			})
		},
		CheckStatus: func(pull forge.PullRequest, head string) (forge.CI, error) {
			return ask(connect, func(on forgeConnection) (forge.CI, error) {
				return on.client.CheckStatus(ctx, on.repo, pull, head)
			})
		},
		JobLog: func(check forge.Check) (forge.JobLog, error) {
			return ask(connect, func(on forgeConnection) (forge.JobLog, error) { return on.client.JobLog(ctx, on.repo, check) })
		},
		Rerun: func(pull forge.PullRequest, head string) (bool, error) {
			return ask(connect, func(on forgeConnection) (bool, error) {
				return on.client.RerunChecks(ctx, on.repo, pull, head)
			})
		},
		Merge: func(pull forge.PullRequest, method forge.MergeMethod) error {
			return tell(connect, func(on forgeConnection) error { return on.client.Merge(ctx, on.repo, pull, method) })
		},
		MergeMethods: func() ([]forge.MergeMethod, error) {
			return ask(connect, func(on forgeConnection) ([]forge.MergeMethod, error) {
				return on.client.MergeMethods(ctx, on.repo)
			})
		},
		ReviewRequests: func() ([]forge.ReviewRequest, error) {
			return ask(connect, func(on forgeConnection) ([]forge.ReviewRequest, error) {
				return on.client.ReviewRequests(ctx, on.repo.Kind)
			})
		},
		Templates:    func() []forge.Template { return templatesFor(setup.settings(), setup.where) },
		Author:       func() (string, error) { return ask(connect, authorName(ctx)) },
		GroupMembers: groupMembersSeam(ctx, kind, connect),
		IsGroup:      isGroupSeam(ctx, kind, connect),
		Kind:         kind,
	}
}

// authorName asks the forge whom the credential its requests carry belongs to,
// by the name the forge shows them by.
func authorName(ctx context.Context) func(forgeConnection) (string, error) {
	return func(on forgeConnection) (string, error) {
		identity, err := on.client.Whoami(ctx)

		return identity.Name(), err
	}
}

// groupMembersSeam is the list-a-group's-members seam, bound only on GitLab:
// GitHub has no groups to list, and its teams review as teams.
func groupMembersSeam(
	ctx context.Context, kind forge.Kind, connect func() (forgeConnection, error),
) func(string) ([]string, error) {
	if kind != forge.KindGitLab {
		return nil
	}

	return func(group string) ([]string, error) {
		return ask(connect, func(on forgeConnection) ([]string, error) { return on.client.GroupMembers(ctx, group) })
	}
}

// isGroupSeam is the is-this-name-a-group seam, bound only on GitLab, where
// CODEOWNERS spells a top-level group as it spells a user. What GitLab
// answered is kept for the session: the web reads the owners at its preview
// and again at its post, and a lookup failing only the second time must not
// turn a group into a person between them.
func isGroupSeam(
	ctx context.Context, kind forge.Kind, connect func() (forgeConnection, error),
) func(string) (bool, error) {
	if kind != forge.KindGitLab {
		return nil
	}

	known := groupsKnown{mutex: &sync.Mutex{}, answers: map[string]bool{}}

	return func(name string) (bool, error) {
		group, found := known.answer(name)
		if found {
			return group, nil
		}

		group, err := ask(connect, func(on forgeConnection) (bool, error) { return on.client.IsGroup(ctx, name) })
		if err == nil {
			known.keep(name, group)
		}

		return group, err
	}
}

// groupsKnown is what the forge answered of each bare name, by its lower
// case, since GitLab reads names without regard to case.
type groupsKnown struct {
	mutex   *sync.Mutex
	answers map[string]bool
}

// answer is what the forge answered of name, and whether it was asked.
func (k groupsKnown) answer(name string) (bool, bool) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	group, found := k.answers[strings.ToLower(name)]

	return group, found
}

// keep records what the forge answered of name.
func (k groupsKnown) keep(name string, group bool) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.answers[strings.ToLower(name)] = group
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

// onceConnected caches a connection once it succeeds, and retries after a
// failure rather than remembering it: a token added, or gh signed in, in another
// terminal is found on the next attempt instead of only on a restart.
func onceConnected[T any](connect func() (T, error)) func() (T, error) {
	var (
		lock   sync.Mutex
		cached T
		ok     bool
	)

	return func() (T, error) {
		lock.Lock()
		defer lock.Unlock()

		if ok {
			return cached, nil
		}

		connection, err := connect()
		if err != nil {
			var none T

			return none, err
		}

		cached, ok = connection, true

		return cached, nil
	}
}

// ask connects with connect, then asks the connection what question does: the
// one shape every seam over a client found on first use takes, Jira's,
// Taskwarrior's and the forge's alike.
func ask[C, T any](connect func() (C, error), question func(C) (T, error)) (T, error) {
	connection, err := connect()
	if err != nil {
		var none T

		return none, err
	}

	return question(connection)
}

// tell connects with connect, then applies change to the connection.
func tell[C any](connect func() (C, error), change func(C) error) error {
	connection, err := connect()
	if err != nil {
		return err
	}

	return change(connection)
}

// liveForge is the forge settings every forge call reads: those workflow
// started with, until the web's Settings saves others.
type liveForge struct {
	lock     sync.Mutex
	settings config.Forge
}

// current is the settings in effect now.
func (l *liveForge) current() config.Forge {
	l.lock.Lock()
	defer l.lock.Unlock()

	return l.settings
}

// replace puts settings in effect for every call after it.
func (l *liveForge) replace(settings config.Forge) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.settings = settings
}

// useForgeSettings is Controls.UseForgeSettings over live and the workspace.
func useForgeSettings(live *liveForge, where Workspace) func(config.Forge) forge.Kind {
	return func(settings config.Forge) forge.Kind {
		live.replace(settings)

		return ForgeKind(settings, where.Remote)
	}
}

// connectedWith caches a connection made with the settings in effect, and makes
// a new one once they change — a token, host or kind saved in the web's
// Settings reaches the next call rather than the next start. Like
// onceConnected, it retries after a failure rather than remembering it.
func connectedWith[T any](current func() config.Forge, connect func(config.Forge) (T, error)) func() (T, error) {
	var (
		lock      sync.Mutex
		cached    T
		ok        bool
		cachedFor config.Forge
	)

	return func() (T, error) {
		lock.Lock()
		defer lock.Unlock()

		settings := current()
		if ok && settings == cachedFor {
			return cached, nil
		}

		connection, err := connect(settings)
		if err != nil {
			var none T

			ok = false

			return none, err
		}

		cached, cachedFor, ok = connection, settings, true

		return cached, nil
	}
}

// connectForge finds the forge the workspace's origin points at and the token
// for it, the same way doctor --online does.
func connectForge(ctx context.Context, setup forgeSetup, settings config.Forge) (forgeConnection, error) {
	repo, err := resolveRepo(settings, setup.where.Remote)
	if err != nil {
		return forgeConnection{}, fmt.Errorf("reading origin: %w", err)
	}

	base, err := repo.APIBase()
	if err != nil {
		return forgeConnection{}, fmt.Errorf("%s — set forge.kind and forge.host: %w", repo.Host, err)
	}

	access, err := ReachForge(ctx, settings, repo, base, setup.httpTransport)
	if err != nil {
		return forgeConnection{}, err
	}

	//nolint:bodyclose // Wrap only relays the response; the forge client reads and closes its body.
	client := forge.New(setup.log.Wrap("forge", access.Doer), base, access.Token).On(repo.Kind)

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
// credential they carry: through the forge's CLI when forge.cli routes them
// there, and otherwise over httpTransport, the redirect-refusing client the
// caller built with httpx.Client. A forge reached through its CLI needs no
// token, since the CLI signs every request with the login it already holds.
// The commands and doctor --online both reach the forge through here, so the
// two cannot come to reach it differently.
func ReachForge(
	ctx context.Context, settings config.Forge, repo forge.Repo, base string, httpTransport httpx.Doer,
) (ForgeAccess, error) {
	transport, usingCLI := forgeTransport(ctx, settings, repo, base, httpTransport)
	if usingCLI {
		program, _ := forgeProgram(repo.Kind)

		// The CLI transport authenticates itself, so no credential is looked
		// for; a placeholder satisfies the client's token guard.
		return ForgeAccess{Doer: transport, Token: cliToken, Via: "through " + program}, nil
	}

	token, source, err := ForgeResolver(settings).Resolve(ctx, repo.Kind, repo.Host)
	if err != nil {
		return ForgeAccess{}, err
	}

	return ForgeAccess{Doer: transport, Token: token, Via: "token from " + source.String()}, nil
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
