// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/lifetime"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// Deps is everything the interface asks of the world outside the terminal,
// grouped by what it asks. Each is a function rather than a client, so the model
// never holds a credential, and a test can hand it canned answers without a
// network, a repository or a subprocess. A function that does I/O is asked
// under the session's context, which Model.WithContext sets; New has each
// write ignore the session's end, and Model.WithLifetime ends it with the
// program's lifetime instead.
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

// outlasting is deps with every write asked under a context outlive makes
// from its caller's, which goes on when the caller's ends: each seam that
// changes something outside the terminal (what a dry run holds back, the
// store's records and a fetch), and the opening of a page in the browser. The
// session ends when the program quits or switches directory, which stops every
// read it left in flight; a write cut off there could be left half made, git's
// index locked by an add stopped midway, so it goes on, as a web write
// outlasts its page, until outlive ends it. A nil seam, one unavailable here,
// stays nil.
func outlasting(outlive outliving, deps Deps, noun string) Deps {
	// Trade-off TRADE-38: the session's end stops reads alone; a write goes on,
	// past a quit, until the program's lifetime ends.
	deps.Jira = outlastingJira(outlive, deps.Jira, noun)
	deps.Git = outlastingGit(outlive, deps.Git)
	deps.Forge = outlastingForge(outlive, deps.Forge, noun)
	deps.Store = outlastingStore(outlive, deps.Store)
	deps.Tasks = outlastingTasks(outlive, deps.Tasks)
	deps.Settings = outlastingSettings(outlive, deps.Settings)
	deps.Messaging.Post = outlasting2(outlive, "posting a message", deps.Messaging.Post)
	deps.Hooks.Run = outlastingRun1(outlive, "running the hooks", deps.Hooks.Run)
	deps.Hooks.Write = outlasting1(outlive, "setting up lefthook", deps.Hooks.Write)
	deps.OpenURL = outlasting1(outlive, "opening a page in the browser", deps.OpenURL)

	return deps
}

// outlastingJira is the tracker with its writes outlasting the session.
func outlastingJira(outlive outliving, tracker seams.Jira, noun string) seams.Jira {
	tracker.Transition = outlasting3(outlive, "moving an issue", tracker.Transition)
	tracker.Comment = outlastingAnswer2(outlive, "commenting on an issue", tracker.Comment)
	tracker.Assign = outlasting2(outlive, "assigning an issue", tracker.Assign)
	tracker.AddWorklog = outlastingAnswer3(outlive, "logging work on an issue", tracker.AddWorklog)
	tracker.LinkPullRequest = outlasting3(outlive, "linking an issue to its "+noun, tracker.LinkPullRequest)

	return tracker
}

// outlastingGit is the repository with its writes outlasting the session.
func outlastingGit(outlive outliving, git seams.Git) seams.Git {
	git.Stage = outlasting1(outlive, "staging a change", git.Stage)
	git.Unstage = outlasting1(outlive, "unstaging a change", git.Unstage)
	git.Discard = outlasting1(outlive, "discarding a change", git.Discard)
	git.CreateBranch = outlasting2(outlive, "creating a branch", git.CreateBranch)
	git.Checkout = outlasting1(outlive, "checking out a branch", git.Checkout)
	git.CreateWorktree = outlastingAnswer2(outlive, "adding a worktree", git.CreateWorktree)
	git.Fetch = outlasting0(outlive, "fetching", git.Fetch)
	git.Commit = outlastingRun1(outlive, "committing", git.Commit)
	git.Push = outlastingRun1(outlive, "pushing", git.Push)
	git.Amend = outlastingRun0(outlive, "amending the last commit", git.Amend)
	git.Fixup = outlastingRun1(outlive, "making a fixup commit", git.Fixup)
	git.Rebase = outlastingRun1(outlive, "rebasing", git.Rebase)
	git.Finish = outlasting2(outlive, "finishing the branch", git.Finish)
	git.LinkIssue = outlasting2(outlive, "linking the branch to an issue", git.LinkIssue)
	git.UnlinkIssue = outlasting1(outlive, "unlinking the branch from its issue", git.UnlinkIssue)

	return git
}

// outlastingForge is the forge with its writes outlasting the session.
func outlastingForge(outlive outliving, forge seams.Forge, noun string) seams.Forge {
	forge.CreatePullRequest = outlastingAnswer1(outlive, "opening a "+noun, forge.CreatePullRequest)
	forge.EditPullRequest = outlastingAnswer2(outlive, "editing the "+noun, forge.EditPullRequest)
	forge.RewriteDescription = outlastingAnswer2(outlive, "rewriting the "+noun+"'s description", forge.RewriteDescription)
	forge.Rerun = outlastingAnswer2(outlive, "rerunning a check", forge.Rerun)
	forge.Merge = outlasting2(outlive, "merging the "+noun, forge.Merge)

	return forge
}

// outlastingStore is the store with its records outlasting the session.
func outlastingStore(outlive outliving, store seams.Store) seams.Store {
	store.RecordScope = outlasting1(outlive, "remembering the commit scope", store.RecordScope)
	store.RecordAnnounce = outlasting1(outlive, "remembering an announcement", store.RecordAnnounce)
	store.CacheIssues = outlasting2(outlive, "caching the issue list", store.CacheIssues)
	store.LinkOwner = outlasting2(outlive, "linking a code owner", store.LinkOwner)
	store.ForgetOwner = outlasting2(outlive, "forgetting a code owner's link", store.ForgetOwner)
	store.SetRepoGroups = outlasting2(outlive, "setting the repository's groups", store.SetRepoGroups)
	store.RecordGroups = outlasting2(outlive, "remembering the groups tagged", store.RecordGroups)
	store.Favor = outlasting1(outlive, "adding a favorite", store.Favor)
	store.Unfavor = outlasting1(outlive, "removing a favorite", store.Unfavor)

	return store
}

// outlastingTasks is Taskwarrior with its writes outlasting the session.
func outlastingTasks(outlive outliving, tasks seams.Tasks) seams.Tasks {
	tasks.Add = outlastingAnswer1(outlive, "adding a task", tasks.Add)
	tasks.Start = outlasting1(outlive, "starting a task", tasks.Start)
	tasks.Stop = outlasting1(outlive, "stopping a task", tasks.Stop)
	tasks.Done = outlasting1(outlive, "marking a task done", tasks.Done)
	tasks.Annotate = outlasting2(outlive, "annotating a task", tasks.Annotate)
	tasks.Modify = outlasting2(outlive, "modifying a task", tasks.Modify)
	tasks.Undo = outlastingAnswer0(outlive, "undoing a task change", tasks.Undo)
	tasks.Sync = outlastingAnswer0(outlive, "syncing tasks", tasks.Sync)

	return tasks
}

// outlastingSettings is the settings with saving them, and writing a first
// file, outlasting the session.
func outlastingSettings(outlive outliving, settings seams.Settings) seams.Settings {
	settings.Setup.Write = outlastingAnswer1(outlive, "writing the configuration file", settings.Setup.Write)

	save := settings.Save
	if save == nil {
		return settings
	}

	settings.Save = func(
		ctx context.Context, edited config.Config, removed []config.Credential, over config.Revision,
	) (config.Config, config.Revision, error) {
		outlives, release := outlive(ctx, "saving settings")
		defer release()

		return save(outlives, edited, removed, over)
	}

	return settings
}

// outliving is how a write's context outlives its caller's: the context to
// write under, with ctx's values and not its end, for the write name says,
// and the function that releases it once the write is done.
type outliving func(ctx context.Context, name string) (context.Context, context.CancelFunc)

// detached is a write's context never ending, with nothing to release: New's,
// until WithLifetime gives the writes an end.
func detached(ctx context.Context, _ string) (context.Context, context.CancelFunc) {
	return context.WithoutCancel(ctx), func() {}
}

// endingWith is a write's context ending when program does, the write counted
// in flight, by its name, in what program counts its work in until it is
// released, so the command line can wait for it before it exits.
func endingWith(program context.Context) outliving {
	inFlight := lifetime.CountedIn(program)

	return func(ctx context.Context, name string) (context.Context, context.CancelFunc) {
		landed := inFlight.Begin(name)
		outlives, cancel := context.WithCancel(context.WithoutCancel(ctx))
		stop := context.AfterFunc(program, cancel)

		return outlives, func() {
			stop()
			cancel()
			landed()
		}
	}
}

// The outlasting functions below are a write of one shape, named by name,
// asked under a context outliving its caller's, released once the write
// returns: each is named for how many arguments follow the context, the
// Answer ones for a write that answers a value beside its error, and the Run
// ones for a program whose output streams on after it returns, which holds
// its context, and counts in flight, until it exits. A nil write stays nil.

func outlasting0(outlive outliving, name string, write func(context.Context) error) func(context.Context) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives)
	}
}

func outlasting1[A any](
	outlive outliving, name string, write func(context.Context, A) error,
) func(context.Context, A) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first)
	}
}

func outlasting2[A, B any](
	outlive outliving, name string, write func(context.Context, A, B) error,
) func(context.Context, A, B) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second)
	}
}

func outlasting3[A, B, C any](
	outlive outliving, name string, write func(context.Context, A, B, C) error,
) func(context.Context, A, B, C) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second, third)
	}
}

func outlastingAnswer0[R any](
	outlive outliving, name string, write func(context.Context) (R, error),
) func(context.Context) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives)
	}
}

func outlastingAnswer1[A, R any](
	outlive outliving, name string, write func(context.Context, A) (R, error),
) func(context.Context, A) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first)
	}
}

func outlastingAnswer2[A, B, R any](
	outlive outliving, name string, write func(context.Context, A, B) (R, error),
) func(context.Context, A, B) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second)
	}
}

func outlastingAnswer3[A, B, C, R any](
	outlive outliving, name string, write func(context.Context, A, B, C) (R, error),
) func(context.Context, A, B, C) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second, third)
	}
}

func outlastingRun0(
	outlive outliving, name string, run func(context.Context) (proc.Output, error),
) func(context.Context) (proc.Output, error) {
	if run == nil {
		return nil
	}

	return func(ctx context.Context) (proc.Output, error) {
		outlives, release := outlive(ctx, name)
		output, err := run(outlives)

		return releasedOnExit(output, err, release)
	}
}

func outlastingRun1[A any](
	outlive outliving, name string, run func(context.Context, A) (proc.Output, error),
) func(context.Context, A) (proc.Output, error) {
	if run == nil {
		return nil
	}

	return func(ctx context.Context, first A) (proc.Output, error) {
		outlives, release := outlive(ctx, name)
		output, err := run(outlives, first)

		return releasedOnExit(output, err, release)
	}
}

// releasedOnExit is a run's output releasing the context it was started
// under once Wait has said how its program exited, or at once when it never
// started: releasing it before would kill the program.
func releasedOnExit(output proc.Output, err error, release context.CancelFunc) (proc.Output, error) {
	if err != nil {
		release()

		return output, err
	}

	wait := output.Wait
	output.Wait = func() error {
		defer release()

		return wait()
	}

	return output, nil
}
