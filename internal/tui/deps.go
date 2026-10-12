// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Deps is everything the interface asks of the world outside the terminal,
// grouped by what it asks. Each is a function rather than a client, so the model
// never holds a credential, and a test can hand it canned answers without a
// network, a repository or a subprocess. A function that does I/O is asked
// under the session's context, which Model.WithContext sets; New has each
// write ignore the session's end, so it goes on to its own.
//
// A load the interface starts by itself is skipped when its function is nil, so
// a test need only supply what it exercises. An action is only offered once the
// model has what it acts on.
//
// The groups every surface shares are declared in package seams. Editor is
// declared here, because it speaks Bubble Tea's messages and commands.
type Deps struct {
	Jira      seams.Jira
	Git       seams.Git
	Forge     seams.Forge
	Messaging seams.Messaging
	Hooks     seams.Hooks
	Editor    EditorDeps
	Store     seams.Store
	// Tasks is nil-fielded when no task program is found or the integration is
	// disabled; see seams.Tasks.
	Tasks        seams.Tasks
	Repositories seams.Repositories
	// Settings reads and saves the configuration files, and lists and removes
	// the local data.
	Settings seams.Settings
	// Clock tells the time, for how long ago a comment was written. Nil means
	// time.Now.
	Clock func() time.Time
	// CIInterval is how often CI is asked about while it runs, or while a post
	// waits for it to pass. Zero means every twenty seconds.
	CIInterval time.Duration
	// After is the timer every wait goes through — the selection resting before
	// an issue is read, the gap between CI checks: a command that delivers fire's
	// message once the wait has passed. Nil means tea.Tick.
	After func(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd
	// Notify rings the terminal and sends a desktop notification, for when CI
	// finishes while the developer is looking elsewhere. Nil where the interface
	// cannot reach the terminal to ring it.
	Notify func()
	// OpenURL opens a page in the user's browser, for a check whose detail lives
	// on the forge. Nil where there is no opener to reach.
	OpenURL func(ctx context.Context, url string) error
	// Copy puts text on the system clipboard. In production it is tea.SetClipboard,
	// which writes it through the terminal's own OSC 52 sequence — no program, and
	// it works over SSH. Nil where the interface cannot reach the terminal.
	Copy func(text string) tea.Cmd
}

// EditorDeps hands text and files to the user's editor, which takes the
// terminal while it is open.
//
// Trade-off TRADE-11: it stays in this package, not in package seams, because
// its functions take and return Bubble Tea's messages and commands.
type EditorDeps struct {
	Edit func(text, help string, done func(string, error) tea.Msg) tea.Cmd
	Open func(file string, line int, done func(error) tea.Msg) tea.Cmd
	// Resolve turns the places a tool printed into the files that open them,
	// keyed by the place as printed and leaving out any that names none. A place
	// relative to a package, not the root, is the common miss. It can walk the
	// whole checkout, so it runs in a command, never in Update.
	Resolve func(places []string) map[string]string
}

// resolvedFailures keeps only the places that resolve to a file, rewriting each
// to the path that opens it, so a place a tool printed relative to its package
// is either found below the root or dropped rather than offered as a jump that
// opens nothing. Every place is resolved in one call, so a run's places cost
// one walk of the checkout between them. With no Resolve seam the places are
// left as they came.
func (d Deps) resolvedFailures(found []hooks.Location) []hooks.Location {
	if d.Editor.Resolve == nil {
		return found
	}

	printed := make([]string, 0, len(found))
	for _, place := range found {
		printed = append(printed, place.File)
	}

	files := d.Editor.Resolve(printed)
	kept := make([]hooks.Location, 0, len(found))

	for _, place := range found {
		file, ok := files[place.File]
		if ok {
			place.File = file
			kept = append(kept, place)
		}
	}

	return kept
}

// ciInterval is how often CI is asked about.
func (d Deps) ciInterval() time.Duration {
	if d.CIInterval <= 0 {
		return config.DefaultCIInterval
	}

	return d.CIInterval
}

// now is the time by the clock the interface was given.
func (d Deps) now() time.Time {
	if d.Clock == nil {
		return time.Now()
	}

	return d.Clock()
}

// after is a command that delivers fire's message once wait has passed, on the
// timer the interface was given.
func (d Deps) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	if d.After == nil {
		return tea.Tick(wait, fire)
	}

	return d.After(wait, fire)
}

// outlasting is deps with every write asked under its caller's context's
// values but never its end: each seam that changes something outside the
// terminal (what a dry run holds back, the store's records and a fetch), and
// the opening of a page in the browser. The session ends when the program
// quits or switches directory, which stops every read it left in flight; a
// write cut off there could be left half made, git's index locked by an add
// stopped midway, so it goes on to its end, as a web write outlasts its page.
// A nil seam, one unavailable here, stays nil.
func outlasting(deps Deps) Deps {
	// Trade-off TRADE-38: the session's end stops reads alone; a write goes on
	// to its end, past a quit.
	deps.Jira = outlastingJira(deps.Jira)
	deps.Git = outlastingGit(deps.Git)
	deps.Forge = outlastingForge(deps.Forge)
	deps.Store = outlastingStore(deps.Store)
	deps.Tasks = outlastingTasks(deps.Tasks)
	deps.Settings = outlastingSettings(deps.Settings)
	deps.Messaging.Post = outlasting2(deps.Messaging.Post)
	deps.Hooks.Run = outlastingAnswer1(deps.Hooks.Run)
	deps.Hooks.Write = outlasting1(deps.Hooks.Write)
	deps.OpenURL = outlasting1(deps.OpenURL)

	return deps
}

// outlastingJira is the tracker with its writes outlasting the session.
func outlastingJira(tracker seams.Jira) seams.Jira {
	tracker.Transition = outlasting3(tracker.Transition)
	tracker.Comment = outlastingAnswer2(tracker.Comment)
	tracker.Assign = outlasting2(tracker.Assign)
	tracker.AddWorklog = outlastingAnswer3(tracker.AddWorklog)
	tracker.LinkPullRequest = outlasting3(tracker.LinkPullRequest)

	return tracker
}

// outlastingGit is the repository with its writes outlasting the session.
func outlastingGit(git seams.Git) seams.Git {
	git.Stage = outlasting1(git.Stage)
	git.Unstage = outlasting1(git.Unstage)
	git.Discard = outlasting1(git.Discard)
	git.CreateBranch = outlasting2(git.CreateBranch)
	git.Checkout = outlasting1(git.Checkout)
	git.CreateWorktree = outlastingAnswer2(git.CreateWorktree)
	git.Fetch = outlasting0(git.Fetch)
	git.Commit = outlastingAnswer1(git.Commit)
	git.Push = outlastingAnswer1(git.Push)
	git.Amend = outlastingAnswer0(git.Amend)
	git.Fixup = outlastingAnswer1(git.Fixup)
	git.Rebase = outlastingAnswer1(git.Rebase)
	git.Finish = outlasting2(git.Finish)
	git.LinkIssue = outlasting2(git.LinkIssue)
	git.UnlinkIssue = outlasting1(git.UnlinkIssue)

	return git
}

// outlastingForge is the forge with its writes outlasting the session.
func outlastingForge(forge seams.Forge) seams.Forge {
	forge.CreatePullRequest = outlastingAnswer1(forge.CreatePullRequest)
	forge.EditPullRequest = outlastingAnswer2(forge.EditPullRequest)
	forge.RewriteDescription = outlastingAnswer2(forge.RewriteDescription)
	forge.Rerun = outlastingAnswer2(forge.Rerun)
	forge.Merge = outlasting2(forge.Merge)

	return forge
}

// outlastingStore is the store with its records outlasting the session.
func outlastingStore(store seams.Store) seams.Store {
	store.RecordScope = outlasting1(store.RecordScope)
	store.RecordAnnounce = outlasting1(store.RecordAnnounce)
	store.CacheIssues = outlasting2(store.CacheIssues)
	store.LinkOwner = outlasting2(store.LinkOwner)
	store.ForgetOwner = outlasting2(store.ForgetOwner)
	store.SetRepoGroups = outlasting2(store.SetRepoGroups)
	store.RecordGroups = outlasting2(store.RecordGroups)
	store.Favor = outlasting1(store.Favor)
	store.Unfavor = outlasting1(store.Unfavor)

	return store
}

// outlastingTasks is Taskwarrior with its writes outlasting the session.
func outlastingTasks(tasks seams.Tasks) seams.Tasks {
	tasks.Add = outlastingAnswer1(tasks.Add)
	tasks.Start = outlasting1(tasks.Start)
	tasks.Stop = outlasting1(tasks.Stop)
	tasks.Done = outlasting1(tasks.Done)
	tasks.Annotate = outlasting2(tasks.Annotate)
	tasks.Modify = outlasting2(tasks.Modify)
	tasks.Undo = outlastingAnswer0(tasks.Undo)
	tasks.Sync = outlastingAnswer0(tasks.Sync)

	return tasks
}

// outlastingSettings is the settings with saving them, and writing a first
// file, outlasting the session.
func outlastingSettings(settings seams.Settings) seams.Settings {
	settings.Setup.Write = outlastingAnswer1(settings.Setup.Write)

	save := settings.Save
	if save == nil {
		return settings
	}

	settings.Save = func(
		ctx context.Context, edited config.Config, removed []config.Credential, over config.Revision,
	) (config.Config, config.Revision, error) {
		return save(context.WithoutCancel(ctx), edited, removed, over)
	}

	return settings
}

// The outlasting functions below are a write of one shape asked under its
// context's values but never its end: each is named for how many arguments
// follow the context, and the Answer ones for a write that answers a value
// beside its error. A nil write stays nil.

func outlasting0(write func(context.Context) error) func(context.Context) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) error { return write(context.WithoutCancel(ctx)) }
}

func outlasting1[A any](write func(context.Context, A) error) func(context.Context, A) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) error { return write(context.WithoutCancel(ctx), first) }
}

func outlasting2[A, B any](write func(context.Context, A, B) error) func(context.Context, A, B) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) error {
		return write(context.WithoutCancel(ctx), first, second)
	}
}

func outlasting3[A, B, C any](write func(context.Context, A, B, C) error) func(context.Context, A, B, C) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) error {
		return write(context.WithoutCancel(ctx), first, second, third)
	}
}

func outlastingAnswer0[R any](write func(context.Context) (R, error)) func(context.Context) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) (R, error) { return write(context.WithoutCancel(ctx)) }
}

func outlastingAnswer1[A, R any](write func(context.Context, A) (R, error)) func(context.Context, A) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) (R, error) { return write(context.WithoutCancel(ctx), first) }
}

func outlastingAnswer2[A, B, R any](
	write func(context.Context, A, B) (R, error),
) func(context.Context, A, B) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) (R, error) {
		return write(context.WithoutCancel(ctx), first, second)
	}
}

func outlastingAnswer3[A, B, C, R any](
	write func(context.Context, A, B, C) (R, error),
) func(context.Context, A, B, C) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) (R, error) {
		return write(context.WithoutCancel(ctx), first, second, third)
	}
}
