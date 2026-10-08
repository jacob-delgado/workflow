// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package wiring connects the seams every surface shares — the command line's,
// the terminal's and the web's — to the real clients that answer them.
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

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// RequestTimeout bounds every request to a service — each doctor --online check,
// and every request the interface makes. Go's http.Client has no default
// deadline, so an unreachable on-prem host would otherwise hang doctor, or leave
// a pane loading forever.
const RequestTimeout = 10 * time.Second

// gitRunner runs git with its terminal prompts turned off. What it runs is quick
// and local — the fetch, pull and push that reach the network stream through
// proc.Start instead — but nothing inside the interface could answer a
// credential prompt, so should one ask, git fails rather than seize the
// terminal. It is the Runner every repository this package builds goes through.
func gitRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return proc.RunCommand(ctx, proc.Command{Name: name, Args: args, Env: []string{"GIT_TERMINAL_PROMPT=0"}})
}

// Deps connects the interface to the real Jira, repository, forge, messaging
// service, lefthook and editor. A non-nil log records the outline of every
// request each service makes. The services share one redirect-refusing HTTP
// client, so a change to how it is built is made here once.
//
// Jira finds its token the first time it is asked, so a command that never
// reaches Jira never runs its token command. The returned Controls'
// ResolveAhead finds it now instead, for the interface and the web server to
// call before they start: once either holds the terminal, a token command that
// asks on it could not be answered. A token not found then is looked for again
// on first use, where its failure is reported. The Slack user token is asked
// for on every post, refreshed as it runs out.
func Deps(ctx context.Context, cfg config.Config, where Workspace, log *RequestLog) (tui.Deps, Controls) {
	httpTransport := httpx.Client(requestTimeout(cfg)).Do
	settings := &liveForge{settings: cfg.Forge}
	setup := forgeSetup{settings: settings.current, where: where, httpTransport: httpTransport, log: log}
	connect := connectedWith(settings.current, func(current config.Forge) (forgeConnection, error) {
		return connectForge(ctx, setup, current)
	})
	jiraClient := onceConnected(func() (jira.Client, error) { return connectJira(ctx, cfg.Jira, httpTransport, log) })
	messagingSettings := &liveMessaging{settings: cfg.Messaging}
	messagingSet := messagingSetup{
		settings: messagingSettings.current, files: cfg.Layers(), httpTransport: httpTransport, log: log,
	}
	directory := NewSlackDirectory(slackUserClient(messagingSet), time.Now)

	// A failure here is left for first use, which looks again and reports it.
	// The Slack user token needs no finding ahead: no command of the user's is
	// run for it, so none can ask on the terminal once the interface holds it.
	resolveAhead := func() {
		if cfg.Jira.Configured() {
			_, _ = jiraClient()
		}
	}

	controls := Controls{
		ResolveAhead: resolveAhead, UseForgeSettings: useForgeSettings(settings, where),
		UseMessagingSettings: func(settings config.Messaging) {
			messagingSettings.replace(settings)
			// What was read belongs to the old settings' workspace and token.
			directory.Refresh()
		},
		//nolint:bodyclose // Wrap only relays the response; the refresh reads and closes its body.
		PlaceSlackCredentials: PlaceSlackCredentials(ctx, log.Wrap("slack", httpTransport), SystemKeychain()),
	}

	deps := tui.Deps{
		Jira:         trackerDeps(ctx, cfg, jiraClient, connect),
		Git:          gitDeps(ctx, where.Root, func() forge.Kind { return ForgeKind(settings.current(), where.Remote) }),
		Forge:        forgeDeps(ctx, setup, connect),
		Messaging:    messagingDeps(ctx, messagingSet, directory),
		Hooks:        hookDeps(ctx, where.Root),
		Editor:       editorDeps(where.Root),
		Store:        storeDeps(ctx, onDisk(cfg), cfg, where),
		Tasks:        taskDeps(ctx, cfg.Taskwarrior),
		Repositories: repositoriesDeps(ctx, cfg, where),
		Settings:     settingsDeps(ctx, cfg.Layers(), controls.PlaceSlackCredentials),
		Clock:        nil,
		CIInterval:   cfg.CIInterval(),
		Notify:       ringTerminal,
		OpenURL:      func(url string) error { return openInBrowser(ctx, url) },
		Copy:         tea.SetClipboard,
	}
	// Favorites are read from the store as it is, never made, so a dry run's
	// Summary reads them as well.
	deps.Git.CommitsBetween = yourCommits(ctx, where, readFavorites(ctx, onDisk(cfg).ReadOnly()))

	return deps, controls
}

// Controls are what a surface asks of the wiring itself, beside the seams.
type Controls struct {
	// ResolveAhead finds the Jira token now, rather than on first use.
	ResolveAhead func()
	// UseForgeSettings applies forge settings saved while workflow runs — the
	// web's Settings — to every forge call after it, and reports which forge the
	// remote is on under them. The terminal's Settings applies what it saves
	// from the next start, and never calls it.
	UseForgeSettings func(settings config.Forge) forge.Kind
	// UseMessagingSettings applies messaging settings saved while workflow runs
	// — the web's Settings — to every post and Slack directory read after it,
	// dropping what the directory read under the old ones.
	UseMessagingSettings func(settings config.Messaging)
	// PlaceSlackCredentials keeps a Slack user token's secrets, typed into the
	// web's Settings or the terminal's, where the configuration keeps them: on macOS it refreshes
	// the token once, saves the pair to the keychain, and answers the
	// configuration without them; elsewhere the file keeps them, and it answers
	// the configuration unchanged.
	PlaceSlackCredentials func(cfg config.Config) (config.Config, error)
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

	_, err = proc.RunCommand(ctx, BrowserCommand(runtime.GOOS, target))
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

// BrowserCommand is the command that opens target, a web URL, on goos: open on
// macOS, rundll32 with url.dll,FileProtocolHandler on Windows, and xdg-open on
// every other platform, each handed the URL as a single argument.
func BrowserCommand(goos, target string) proc.Command {
	switch goos {
	case "darwin":
		return proc.Command{Name: "open", Args: []string{target}}
	case "windows":
		return proc.Command{Name: "rundll32", Args: []string{"url.dll,FileProtocolHandler", target}}
	default:
		return proc.Command{Name: "xdg-open", Args: []string{target}}
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

// gitDeps is what a surface asks of the repository. CODEOWNERS is read in the
// dialect of the forge kind names at the time of the read, since Settings can
// change the forge while workflow runs.
func gitDeps(ctx context.Context, root string, kind func() forge.Kind) seams.Git {
	repo := gitrepo.At(gitRunner, root)

	return seams.Git{
		Branch:         func() (gitrepo.Branch, error) { return repo.ReadBranch(ctx) },
		Changes:        func() ([]gitrepo.Change, error) { return repo.Status(ctx) },
		Diff:           func(change gitrepo.Change) ([]string, error) { return repo.Diff(ctx, change) },
		Stage:          func(change gitrepo.Change) error { return repo.Stage(ctx, change) },
		Unstage:        func(change gitrepo.Change) error { return repo.Unstage(ctx, change) },
		Discard:        func(change gitrepo.Change) error { return repo.Discard(ctx, change) },
		CreateBranch:   func(name, start string) error { return repo.CreateBranch(ctx, name, start) },
		Branches:       func() ([]string, error) { return repo.LocalBranches(ctx) },
		Checkout:       func(name string) error { return repo.Checkout(ctx, name) },
		RemoteBranches: func() ([]string, error) { return repo.RemoteBranches(ctx) },
		ChangedPaths:   func(base string) ([]string, error) { return repo.ChangedPaths(ctx, base) },
		CodeOwnersAt: func(base string) (codeowners.File, bool, error) {
			return repo.CodeOwnersAt(ctx, base, codeOwnersDialect(kind()))
		},
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
			return repo.FinishBranch(ctx, branch, base, func() error { return streamToEnd(ctx, gitrepo.PullCommand(root)) })
		},
		IssueLinks:  func() map[string]string { return repo.IssueLinks(ctx) },
		LinkIssue:   func(branch, issueKey string) error { return repo.SetIssueLink(ctx, branch, issueKey) },
		UnlinkIssue: func(branch string) error { return repo.ClearIssueLink(ctx, branch) },
	}
}

// codeOwnersDialect is the CODEOWNERS dialect of a forge kind. A forge it
// cannot name reads as GitHub, whose rules GitLab's extend.
func codeOwnersDialect(kind forge.Kind) codeowners.Dialect {
	if kind == forge.KindGitLab {
		return codeowners.GitLab
	}

	return codeowners.GitHub
}

// streamToEnd runs a network git command unbounded and waits for it to exit. A
// failure keeps what git printed, which is where its reason is: a network
// error, a base that cannot fast-forward, a credential it could not ask for.
func streamToEnd(ctx context.Context, command proc.Command) error {
	output, err := proc.Start(ctx, command)
	if err != nil {
		return err
	}

	var printed []string
	for line := range output.Lines {
		printed = append(printed, line)
	}

	err = output.Wait()
	if err != nil {
		return fmt.Errorf("%w:\n%s", err, sanitize.Text(strings.Join(printed, "\n")))
	}

	return nil
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

// ReadOnlyStore is the store's seams for a dry run: they read what an earlier
// session kept, when the store is already on disk, and never create or write it.
func ReadOnlyStore(ctx context.Context, cfg config.Config, where Workspace) seams.Store {
	return storeDeps(ctx, onDisk(cfg).ReadOnly(), cfg, where)
}

// onDisk is the store under the OS-native data directory, as the configuration
// keeps it or disables it.
func onDisk(cfg config.Config) store.Store {
	dir, _ := store.DefaultDir()

	return store.New(dir, cfg.Store.Disabled)
}

// storeDeps binds the on-disk store to this repository, so the interface can open
// on what was done here before. A store with nowhere to keep its file, or one the
// configuration disabled, no-ops through the same seams, so the interface simply
// learns nothing.
func storeDeps(ctx context.Context, kept store.Store, cfg config.Config, where Workspace) seams.Store {
	repo := repoKey(where)
	instance := instanceKey(cfg.Jira.BaseURL)

	return bindKept(ctx, kept, where, seams.Store{
		LastScope: func() (string, bool) {
			scope, found, _ := kept.LastScope(ctx, repo)

			// The store is a file on disk: what it reads back is untrusted, so
			// neutralize any terminal control it may carry before the composer shows it.
			return sanitize.Line(scope), found
		},
		RecordScope: func(scope string) {
			_ = kept.RecordScope(ctx, repo, sanitize.Line(scope), time.Now())
		},
		Announced: func() []loop.Announced {
			recorded, _ := kept.Announces(ctx, repo)

			made := make([]loop.Announced, 0, len(recorded))
			for _, announce := range recorded {
				made = append(made, loop.Announced{Pull: announce.Pull, Moment: messaging.Moment(announce.Moment)})
			}

			return made
		},
		RecordAnnounce: func(made loop.Announced) {
			_ = kept.RecordAnnounce(ctx, repo, store.Announce{Pull: made.Pull, Moment: int(made.Moment)}, time.Now())
		},
		CachedIssues: func(view string) ([]jira.Issue, bool) {
			cached, found, _ := kept.CachedIssues(ctx, instance, view)
			if !found {
				return nil, false
			}

			return cacheableRows(fromCachedIssues(cached))
		},
		CacheIssues: func(view string, issues []jira.Issue) {
			rows, keep := cacheableRows(issues)
			if keep {
				_ = kept.CacheIssues(ctx, instance, view, toCachedIssues(rows), time.Now())
			}
		},
	})
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

// hookDeps is what a surface asks of lefthook — nothing at all when lefthook
// is not installed, so its actions are not offered.
func hookDeps(ctx context.Context, root string) seams.Hooks {
	if !proc.Available("lefthook") {
		return seams.Hooks{Run: nil, Existing: nil, Write: nil}
	}

	return seams.Hooks{
		Run: func(hook string) (proc.Output, error) { return proc.Start(ctx, hooks.RunCommand(root, hook)) },
		Existing: func() ([]hooks.GitHook, bool) {
			dir, err := gitrepo.At(gitRunner, root).HooksDir(ctx)
			if err != nil {
				return nil, false
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
		Resolve: func(places []string) map[string]string {
			return editor.Resolve(root, os.DirFS(root), places)
		},
	}
}
