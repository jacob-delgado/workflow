// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package wiring connects the terminal interface to the real Jira, repository,
// forge, messaging service, lefthook and editor. It is the one place each seam
// the interface declares meets the client that answers it.
package wiring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/store"
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

// gitRunner runs git with its terminal prompts turned off. Nothing inside the
// interface can answer a credential prompt, so a git command that reaches the
// network — a finish's fast-forward pull — must fail with a reason rather than
// seize the terminal waiting for input that never comes. It is the Runner every
// repository this package builds goes through.
func gitRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return proc.RunCommand(ctx, proc.Command{Name: name, Args: args, Env: []string{"GIT_TERMINAL_PROMPT=0"}})
}

// Locate reads the repository dir is in. Outside one, every git action fails on
// its own and says why, so this is not an error.
func Locate(ctx context.Context, dir string) Workspace {
	repo, err := gitrepo.At(gitRunner, dir).Describe(ctx)
	if err != nil {
		return Workspace{Root: dir, Remote: ""}
	}

	return Workspace{Root: repo.Root, Remote: repo.Remote}
}

// Deps connects the interface to the real Jira, repository, forge, messaging
// service, lefthook and editor. A non-nil log records the outline of every
// request each service makes.
func Deps(ctx context.Context, cfg config.Config, where Workspace, log *RequestLog) tui.Deps {
	timeout := requestTimeout(cfg)

	return tui.Deps{
		Jira:       trackerDeps(ctx, cfg, where, timeout, log),
		Git:        gitDeps(ctx, where.Root),
		Forge:      forgeDeps(ctx, cfg.Forge, where, timeout, log),
		Messaging:  messagingDeps(ctx, cfg.Messaging, timeout, log),
		Hooks:      hookDeps(ctx, where.Root),
		Editor:     editorDeps(where.Root),
		Store:      storeDeps(ctx, cfg, where),
		Clock:      nil,
		CIInterval: cfg.CIInterval(),
		Notify:     ringTerminal,
		OpenURL:    func(url string) error { return openInBrowser(ctx, url) },
		Copy:       tea.SetClipboard,
	}
}

// ciFinished is a terminal bell followed by an OSC 9 desktop notification. A
// terminal that understands OSC 9 raises a system notification; the rest ignore
// it. Neither needs a dependency.
const ciFinished = "\a\x1b]9;CI finished\a"

// ringTerminal rings the terminal CI is being watched from. It writes to the
// same standard output the interface draws on, which is the terminal.
func ringTerminal() {
	_, _ = os.Stdout.WriteString(ciFinished)
}

// errUnsafeBrowserURL is a check's address the opener will not run: one that is
// not web, and so could name a local file or be read as a flag by the opener
// rather than handed to a browser.
var errUnsafeBrowserURL = errors.New("refusing to open a non-http(s) URL")

// openInBrowser opens a URL through the platform's own opener, so a check's page
// on the forge is one keypress away from its line in the interface.
func openInBrowser(ctx context.Context, raw string) error {
	target, err := safeBrowserURL(raw)
	if err != nil {
		return err
	}

	name, args := browserCommand(runtime.GOOS, target)

	_, err = proc.Run(ctx, name, args...)
	if err != nil {
		return fmt.Errorf("opening %s: %w", target, err)
	}

	return nil
}

// safeBrowserURL is raw when it is a web address, or an error otherwise. The URL
// comes from the forge, so requiring http(s) keeps a hostile response from
// naming a local file or slipping a leading dash the opener would read as a flag.
func safeBrowserURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%w: %q", errUnsafeBrowserURL, raw)
	}

	return raw, nil
}

// browserCommand is the command that opens a web URL on goos. It takes the
// platform as an argument rather than reading it, so every branch can be tested
// from one machine.
func browserCommand(goos, target string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		return "cmd", []string{"/c", "start", "", target}
	default:
		return "xdg-open", []string{target}
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
		Search: func(jql string, startAt int) (jira.SearchResult, error) { return client.Search(ctx, jql, startAt) },
		Issue:  func(issueKey jira.Key) (jira.IssueDetail, error) { return client.Issue(ctx, issueKey) },
		Transitions: func(issueKey jira.Key) ([]jira.Transition, error) {
			return client.Transitions(ctx, issueKey)
		},
		Transition: func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error {
			return client.ApplyTransition(ctx, issueKey, to, values)
		},
		Comment: func(issueKey jira.Key, text string) (jira.Comment, error) {
			return client.AddComment(ctx, issueKey, text)
		},
		Assign: func(issueKey jira.Key, assignee string) error {
			return client.Assign(ctx, issueKey, assignee)
		},
		AddWorklog: func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			return client.AddWorklog(ctx, issueKey, timeSpent, comment)
		},
		LinkPullRequest: func(issueKey jira.Key, pullURL, title string) error {
			return client.LinkPullRequest(ctx, issueKey, pullURL, title)
		},
		BrowseURL: client.BrowseURL,
	}
}

// gitDeps is what the interface asks of the repository.
func gitDeps(ctx context.Context, root string) tui.GitDeps {
	repo := gitrepo.At(gitRunner, root)

	return tui.GitDeps{
		Branch:         func() (gitrepo.Branch, error) { return repo.ReadBranch(ctx) },
		Changes:        func() ([]gitrepo.Change, error) { return repo.Status(ctx) },
		Diff:           func(change gitrepo.Change) ([]string, error) { return repo.Diff(ctx, change) },
		Stage:          func(change gitrepo.Change) error { return repo.Stage(ctx, change) },
		Unstage:        func(change gitrepo.Change) error { return repo.Unstage(ctx, change) },
		CreateBranch:   func(name, start string) error { return repo.CreateBranch(ctx, name, start) },
		Branches:       func() ([]string, error) { return repo.LocalBranches(ctx) },
		Checkout:       func(name string) error { return repo.Checkout(ctx, name) },
		RemoteBranches: func() ([]string, error) { return repo.RemoteBranches(ctx) },
		CodeOwners:     func() ([]string, error) { return repo.CodeOwners(ctx) },
		RecentSubjects: func() ([]string, error) { return repo.RecentSubjects(ctx) },
		CreateWorktree: func(name, start string) (string, error) {
			return repo.WorktreeAdd(ctx, name, start)
		},
		Fetch:  func() error { return streamToEnd(ctx, gitrepo.FetchCommand(root)) },
		Commit: func(message string) (proc.Output, error) { return commitWith(ctx, root, message) },
		Amend:  func() (proc.Output, error) { return proc.Start(ctx, gitrepo.AmendCommand(root)) },
		Fixup:  func(hash string) (proc.Output, error) { return proc.Start(ctx, gitrepo.FixupCommand(root, hash)) },
		Push: func(branch string) (proc.Output, error) {
			return proc.Start(ctx, gitrepo.PushCommand(root, repo.PushRemote(ctx), branch))
		},
		Rebase: func(base string) (proc.Output, error) {
			return proc.Start(ctx, gitrepo.RebaseCommand(root, base))
		},
		Finish: func(branch, base string) error {
			return repo.FinishBranch(ctx, branch, base, func() error {
				_, err := proc.RunCommand(ctx, gitrepo.PullCommand(root))

				return err
			})
		},
	}
}

// streamToEnd runs a network git command unbounded, draining its output and
// returning how it exited — a network error, or a credential git could not get
// with prompts off.
func streamToEnd(ctx context.Context, command proc.Command) error {
	output, err := proc.Start(ctx, command)
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

// messagingDeps is what the interface asks of the messaging service. Only a Slack
// bot token is resolved from a command or environment variable; the webhook
// kinds carry the credential in the URL and need no token lookup.
func messagingDeps(
	ctx context.Context, settings config.Messaging, timeout time.Duration, log *RequestLog,
) tui.MessagingDeps {
	// A webhook's path is its credential; only a bot posts to a route.
	wrap := log.wrapWebhook

	if settings.Mode() == config.MessagingBot {
		settings.Token, _, _ = ResolveToken(ctx, settings.Token, settings.TokenCommand, settings.TokenEnv)
		wrap = log.Wrap
	}

	service := strings.ToLower(settings.Service())
	//nolint:bodyclose // wrap only relays the response; the messaging client reads and closes its body.
	client := messaging.New(wrap(service, messaging.HTTPClient(timeout).Do), messaging.APIBase, settings)

	return tui.MessagingDeps{Post: func(channel, text string) error { return client.Post(ctx, channel, text) }}
}

// storeDeps binds the on-disk store to this repository, so the interface can open
// on what was done here before. A store with nowhere to keep its file, or one the
// configuration disabled, no-ops through the same seams, so the interface simply
// learns nothing.
func storeDeps(ctx context.Context, cfg config.Config, where Workspace) tui.StoreDeps {
	dir, _ := store.DefaultDir()
	kept := store.New(dir, cfg.Store.Disabled)
	repo := repoKey(where)
	instance := instanceKey(cfg.Jira.BaseURL)

	return tui.StoreDeps{
		LastScope: func() (string, bool) {
			scope, found, _ := kept.LastScope(ctx, repo)

			// The store is a file on disk: what it reads back is untrusted, so
			// neutralize any terminal control it may carry before the composer shows it.
			return sanitize.Line(scope), found
		},
		RecordScope: func(scope string) {
			_ = kept.RecordScope(ctx, repo, sanitize.Line(scope), time.Now())
		},
		Announced: func() []tui.AnnouncedPost {
			recorded, _ := kept.Announces(ctx, repo)

			posts := make([]tui.AnnouncedPost, 0, len(recorded))
			for _, announce := range recorded {
				posts = append(posts, tui.AnnouncedPost{Pull: announce.Pull, Moment: announce.Moment})
			}

			return posts
		},
		RecordAnnounce: func(post tui.AnnouncedPost) {
			_ = kept.RecordAnnounce(ctx, repo, store.Announce{Pull: post.Pull, Moment: post.Moment}, time.Now())
		},
		CachedIssues: func(view string) ([]jira.Issue, bool) {
			cached, found, _ := kept.CachedIssues(ctx, instance, view)
			if !found {
				return nil, false
			}

			return fromCachedIssues(cached), true
		},
		CacheIssues: func(view string, issues []jira.Issue) {
			_ = kept.CacheIssues(ctx, instance, view, toCachedIssues(issues), time.Now())
		},
	}
}

// toCachedIssues reduces the tracker's issues to the store's shape, neutralizing
// terminal control in each field so nothing hostile is written to the file.
func toCachedIssues(issues []jira.Issue) []store.CachedIssue {
	cached := make([]store.CachedIssue, len(issues))
	for index, issue := range issues {
		cached[index] = store.CachedIssue{
			Key: sanitize.Line(string(issue.Key)), Summary: sanitize.Line(issue.Summary),
			Status: sanitize.Line(issue.Status), StatusCategory: sanitize.Line(string(issue.StatusCategory)),
			Type: sanitize.Line(issue.Type), Priority: sanitize.Line(issue.Priority),
		}
	}

	return cached
}

// fromCachedIssues rebuilds the tracker's issues from the store, sanitizing each
// field again: the store is a file on disk, so what it reads back is untrusted
// and must not reach the terminal as a control sequence.
func fromCachedIssues(cached []store.CachedIssue) []jira.Issue {
	issues := make([]jira.Issue, len(cached))
	for index, issue := range cached {
		issues[index] = jira.Issue{
			Key: jira.Key(sanitize.Line(issue.Key)), Summary: sanitize.Line(issue.Summary),
			Status: sanitize.Line(issue.Status), StatusCategory: jira.StatusCategory(sanitize.Line(issue.StatusCategory)),
			Type: sanitize.Line(issue.Type), Priority: sanitize.Line(issue.Priority),
		}
	}

	return issues
}

// instanceKey identifies a Jira instance for the store without keeping its URL:
// a hash of the base URL, empty when none is configured.
func instanceKey(baseURL string) string {
	if baseURL == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(baseURL))

	return hex.EncodeToString(sum[:])
}

// repoKey names the repository the store keys its state by: the origin remote's
// host and path where there is one, so the same repository shares it across
// clones, and the working tree's root otherwise. The remote is parsed to its
// host and path, never used raw, because an HTTPS remote can carry a credential
// in its userinfo and the store must never hold a secret.
func repoKey(where Workspace) string {
	if where.Remote == "" {
		return where.Root
	}

	repo, err := forge.ParseRemote(where.Remote)
	if err != nil {
		return where.Root
	}

	return repo.Host + "/" + repo.Path
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
			dir, err := gitrepo.At(gitRunner, root).HooksDir(ctx)
			if err != nil {
				return nil, true
			}

			return hooks.ExistingHooks(os.DirFS(dir), runtime.GOOS), hooks.HasConfig(os.DirFS(root))
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
		Resolve: func(file string) (string, bool) {
			return editor.Resolve(root, file)
		},
	}
}
