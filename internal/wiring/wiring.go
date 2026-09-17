// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package wiring connects the terminal interface to the real Jira, repository,
// forge, Slack, lefthook and editor. It is the one place each seam the interface
// declares meets the client that answers it.
package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/slack"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// RequestTimeout bounds every request to a service — each doctor --online check,
// and every request the interface makes. Go's http.Client has no default
// deadline, so an unreachable on-prem host would otherwise hang doctor, or leave
// a pane loading forever.
const RequestTimeout = 10 * time.Second

// Workspace is where the interface runs: the repository's root, or the directory
// it was started in when there is no repository, and origin's URL.
type Workspace struct {
	Root   string
	Remote string
}

// Locate reads the repository dir is in. Outside one, every git action fails on
// its own and says why, so this is not an error.
func Locate(ctx context.Context, dir string) Workspace {
	repo, err := gitrepo.Describe(ctx, proc.Run, dir)
	if err != nil {
		return Workspace{Root: dir, Remote: ""}
	}

	return Workspace{Root: repo.Root, Remote: repo.Remote}
}

// Deps connects the interface to the real Jira, repository, forge, Slack,
// lefthook and editor. A non-nil log records the outline of every request each
// service makes.
func Deps(ctx context.Context, cfg config.Config, where Workspace, log *RequestLog) tui.Deps {
	timeout := requestTimeout(cfg)

	return tui.Deps{
		Jira:       jiraDeps(ctx, cfg.Jira, timeout, log),
		Git:        gitDeps(ctx, where.Root),
		Forge:      forgeDeps(ctx, cfg.Forge, where, timeout, log),
		Slack:      slackDeps(ctx, cfg.Slack, timeout, log),
		Hooks:      hookDeps(ctx, where.Root),
		Editor:     editorDeps(where.Root),
		Clock:      nil,
		CIInterval: cfg.CIInterval(),
	}
}

// requestTimeout is the configured per-request timeout, or the default when none
// is set.
func requestTimeout(cfg config.Config) time.Duration {
	if timeout := cfg.RequestTimeout(); timeout > 0 {
		return timeout
	}

	return RequestTimeout
}

// jiraDeps is what the interface asks of Jira. It needs no guard for a missing
// or malformed configuration: the client refuses before sending anything, and
// the pane shows why.
func jiraDeps(ctx context.Context, settings config.Jira, timeout time.Duration, log *RequestLog) tui.JiraDeps {
	settings.Token, _, _ = ResolveToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
	//nolint:bodyclose // Wrap only relays the response; the jira client reads and closes its body.
	client := jira.New(log.Wrap("jira", jira.HTTPClient(timeout).Do), settings)

	return tui.JiraDeps{
		Search: func(startAt int) (jira.SearchResult, error) { return client.Search(ctx, jira.AssignedToMe, startAt) },
		Issue:  func(issueKey string) (jira.IssueDetail, error) { return client.Issue(ctx, issueKey) },
		Transitions: func(issueKey string) ([]jira.Transition, error) {
			return client.Transitions(ctx, issueKey)
		},
		Transition: func(issueKey string, to jira.Transition, values []jira.FieldValue) error {
			return client.ApplyTransition(ctx, issueKey, to, values)
		},
		Comment: func(issueKey, text string) (jira.Comment, error) {
			return client.AddComment(ctx, issueKey, text)
		},
		LinkPullRequest: func(issueKey, pullURL, title string) error {
			return client.LinkPullRequest(ctx, issueKey, pullURL, title)
		},
		BrowseURL: client.BrowseURL,
	}
}

// gitDeps is what the interface asks of the repository.
func gitDeps(ctx context.Context, root string) tui.GitDeps {
	return tui.GitDeps{
		Branch:  func() (gitrepo.Branch, error) { return gitrepo.ReadBranch(ctx, proc.Run, root) },
		Changes: func() ([]gitrepo.Change, error) { return gitrepo.Status(ctx, proc.Run, root) },
		Stage:   func(change gitrepo.Change) error { return gitrepo.Stage(ctx, proc.Run, root, change) },
		Unstage: func(change gitrepo.Change) error { return gitrepo.Unstage(ctx, proc.Run, root, change) },
		CreateBranch: func(name, start string) error {
			return gitrepo.CreateBranch(ctx, proc.Run, root, name, start)
		},
		Branches: func() ([]string, error) { return gitrepo.LocalBranches(ctx, proc.Run, root) },
		Checkout: func(name string) error { return gitrepo.Checkout(ctx, proc.Run, root, name) },
		CreateWorktree: func(name, start string) (string, error) {
			return gitrepo.WorktreeAdd(ctx, proc.Run, root, name, start)
		},
		Fetch:  func() error { return fetchOrigin(ctx, root) },
		Commit: func(message string) (proc.Output, error) { return commitWith(ctx, root, message) },
		Push: func(branch string) (proc.Output, error) {
			return proc.Start(ctx, gitrepo.PushCommand(root, branch))
		},
	}
}

// fetchOrigin updates origin's tracking refs, draining git's output and
// returning how it exited — a network error, or a credential git could not get
// with prompts off.
func fetchOrigin(ctx context.Context, root string) error {
	output, err := proc.Start(ctx, gitrepo.FetchCommand(root))
	if err != nil {
		return err
	}

	for range output.Lines { //nolint:revive // draining the stream is the point; there is nothing to do per line
	}

	return output.Wait()
}

// commitWith commits with a message written to a private temporary file, which
// is removed once git has exited.
func commitWith(ctx context.Context, root, message string) (proc.Output, error) {
	file, err := os.CreateTemp("", "workflow-commit-*.txt")
	if err != nil {
		return proc.Output{}, fmt.Errorf("writing the commit message: %w", err)
	}

	path := file.Name()
	_, err = file.WriteString(message)

	err = errors.Join(err, file.Close())
	if err != nil {
		_ = os.Remove(path)

		return proc.Output{}, fmt.Errorf("writing the commit message: %w", err)
	}

	output, err := proc.Start(ctx, gitrepo.CommitCommand(root, path))
	if err != nil {
		_ = os.Remove(path)

		return proc.Output{}, fmt.Errorf("starting git commit: %w", err)
	}

	wait := output.Wait
	output.Wait = func() error {
		defer func() { _ = os.Remove(path) }()

		return wait()
	}

	return output, nil
}

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
		Templates: func() []forge.Template { return templatesFor(settings, where) },
		Author: func() (string, error) {
			connection, err := connect()
			if err != nil {
				return "", err
			}

			identity, err := connection.client.Whoami(ctx)

			return identity.Name(), err
		},
		Kind: forgeKind(settings, where.Remote),
	}
}

// forgeKind is which forge the remote points at, read without a network call so
// the interface can name a change correctly from the start. An unparsable remote
// is simply unknown.
func forgeKind(settings config.Forge, remote string) forge.Kind {
	repo, err := forge.ParseRemote(remote)
	if err != nil {
		return forge.KindUnknown
	}

	repo, err = repo.WithConfiguredKind(ForgeSettings(settings))
	if err != nil {
		return forge.KindUnknown
	}

	return repo.Kind
}

// ForgeSettings is the configuration file's say about the forge, as the forge
// package takes it. doctor reads it through here too, so the two cannot come
// to read the file differently.
func ForgeSettings(settings config.Forge) forge.Configured {
	return forge.Configured{Kind: settings.Kind, Host: settings.Host, Token: forge.Token(settings.Token)}
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
	repo, err := forge.ParseRemote(remote)
	if err != nil {
		return forgeConnection{}, fmt.Errorf("reading origin: %w", err)
	}

	repo, err = repo.WithConfiguredKind(ForgeSettings(settings))
	if err != nil {
		return forgeConnection{}, err
	}

	base, err := repo.APIBase()
	if err != nil {
		return forgeConnection{}, fmt.Errorf("%s — set forge.kind and forge.host: %w", repo.Host, err)
	}

	resolver := forge.Resolver{
		Getenv: os.Getenv, Look: proc.LookPath, Run: proc.Run, Configured: ForgeSettings(settings),
	}

	token, _, err := resolver.Resolve(ctx, repo.Kind, repo.Host)
	if err != nil {
		return forgeConnection{}, err
	}

	//nolint:bodyclose // Wrap only relays the response; the forge client reads and closes its body.
	client := forge.New(log.Wrap("forge", forge.HTTPClient(timeout).Do), base, token)

	return forgeConnection{client: client, repo: repo}, nil
}

// templatesFor reads the repository's pull request templates, where its forge
// looks for them.
func templatesFor(settings config.Forge, where Workspace) []forge.Template {
	repo, err := forge.ParseRemote(where.Remote)
	if err != nil {
		return nil
	}

	repo, err = repo.WithConfiguredKind(ForgeSettings(settings))
	if err != nil {
		return nil
	}

	return forge.FindTemplates(os.DirFS(where.Root), repo.Kind)
}

// slackDeps is what the interface asks of Slack.
func slackDeps(ctx context.Context, settings config.Slack, timeout time.Duration, log *RequestLog) tui.SlackDeps {
	settings.Token, _, _ = ResolveToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
	//nolint:bodyclose // Wrap only relays the response; the slack client reads and closes its body.
	client := slack.New(log.Wrap("slack", slack.HTTPClient(timeout).Do), slack.APIBase, settings)

	return tui.SlackDeps{Post: func(channel, text string) error { return client.Post(ctx, channel, text) }}
}

// hookDeps is what the interface asks of lefthook — nothing at all when lefthook
// is not installed, so its actions are not offered.
func hookDeps(ctx context.Context, root string) tui.HookDeps {
	if !proc.Available("lefthook") {
		return tui.HookDeps{Run: nil, Existing: nil, Write: nil}
	}

	return tui.HookDeps{
		Run: func(hook string) (proc.Output, error) { return proc.Start(ctx, hooks.RunCommand(root, hook)) },
		Existing: func() ([]hooks.GitHook, bool) {
			dir, err := gitrepo.HooksDir(ctx, proc.Run, root)
			if err != nil {
				return nil, true
			}

			return hooks.ExistingHooks(os.DirFS(dir)), hooks.HasConfig(os.DirFS(root))
		},
		Write: func(generated hooks.Generated) error {
			err := hooks.Write(root, generated)
			if err != nil {
				return err
			}

			return installLefthook(ctx, root)
		},
	}
}

// installLefthook runs lefthook install in the repository — its root, not
// wherever the process happens to be, which for a test is this repository.
func installLefthook(ctx context.Context, root string) error {
	output, err := proc.Start(ctx, proc.Command{Dir: root, Name: "lefthook", Args: []string{"install"}, Env: nil})
	if err != nil {
		return fmt.Errorf("installing lefthook: %w", err)
	}

	var said []string
	for line := range output.Lines {
		said = append(said, sanitize.Text(line))
	}

	err = output.Wait()
	if err != nil {
		return fmt.Errorf("installing lefthook: %w: %s", err, strings.Join(said, "; "))
	}

	return nil
}

// editorDeps hands text and files to the user's editor.
func editorDeps(root string) tui.EditorDeps {
	return tui.EditorDeps{
		Edit: func(text, help string, done func(string, error) tea.Msg) tea.Cmd {
			return editor.Edit(os.Getenv, root, text, help, done)
		},
		Open: func(file string, line int, done func(error) tea.Msg) tea.Cmd {
			return editor.Open(os.Getenv, root, file, line, done)
		},
	}
}
