// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/lifetime"
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
// The groups every surface shares are declared in package seams. Editor, Forms,
// After and Copy speak Bubble Tea, so the bundle is declared here (TRADE-11).
type Deps struct {
	Jira      seams.Jira
	Git       seams.Git
	Forge     seams.Forge
	Messaging seams.Messaging
	Hooks     seams.Hooks
	Editor    EditorDeps
	// Forms asks a form's questions as plain prompts, when the interface is
	// told to; see FormDeps.
	Forms FormDeps
	Store seams.Store
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
	// announceCI is the forge's CI read an announcement's gate makes as it
	// posts: part of the write, so it outlasts the session as the post does.
	announceCI func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error)
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
// store's records and a fetch). The session ends when the program quits or
// switches directory, which stops every read it left in flight; a write cut
// off there could be left half made, git's index locked by an add stopped
// midway, so it goes on, as a web write outlasts its page, until outlive ends
// it. The opening of a page in the browser goes on past the session too,
// under a context open makes, so a quit does not stop an opener midway, but
// it changes nothing workflow or a forge holds, and nothing waits for it. A
// nil seam, one unavailable here, stays nil.
func outlasting(outlive, open lifetime.Outliving, deps Deps, noun string) Deps {
	// Trade-off TRADE-38: the session's end stops reads alone; a write goes on,
	// past a quit, until the program's lifetime ends.
	deps.Jira = outlastingJira(outlive, deps.Jira, noun)
	deps.Git = outlastingGit(outlive, deps.Git)
	deps.Forge = outlastingForge(outlive, deps.Forge, noun)
	deps.Store = outlastingStore(outlive, deps.Store)
	deps.Tasks = outlastingTasks(outlive, deps.Tasks)
	deps.Settings = outlastingSettings(outlive, deps.Settings)
	deps.Messaging.Post = lifetime.Outlasting2(outlive, "posting a message", deps.Messaging.Post)
	deps.announceCI = lifetime.OutlastingAnswer2(outlive, "reading CI before announcing", deps.Forge.CheckStatus)
	deps.Hooks.Run = lifetime.OutlastingRun1(outlive, "running the hooks", deps.Hooks.Run)
	deps.Hooks.Write = lifetime.Outlasting1(outlive, "setting up lefthook", deps.Hooks.Write)
	deps.OpenURL = lifetime.Outlasting1(open, "opening a page in the browser", deps.OpenURL)

	return deps
}

// outlastingJira is the tracker with its writes outlasting the session.
func outlastingJira(outlive lifetime.Outliving, tracker seams.Jira, noun string) seams.Jira {
	tracker.Transition = lifetime.Outlasting3(outlive, "moving an issue", tracker.Transition)
	tracker.Comment = lifetime.OutlastingAnswer2(outlive, "commenting on an issue", tracker.Comment)
	tracker.Assign = lifetime.Outlasting2(outlive, "assigning an issue", tracker.Assign)
	tracker.AddWorklog = lifetime.OutlastingAnswer3(outlive, "logging work on an issue", tracker.AddWorklog)
	tracker.LinkPullRequest = lifetime.Outlasting3(outlive, "linking an issue to its "+noun, tracker.LinkPullRequest)

	return tracker
}

// outlastingGit is the repository with its writes outlasting the session.
func outlastingGit(outlive lifetime.Outliving, git seams.Git) seams.Git {
	git.Stage = lifetime.Outlasting1(outlive, "staging a change", git.Stage)
	git.Unstage = lifetime.Outlasting1(outlive, "unstaging a change", git.Unstage)
	git.Discard = lifetime.Outlasting1(outlive, "discarding a change", git.Discard)
	git.CreateBranch = lifetime.Outlasting2(outlive, "creating a branch", git.CreateBranch)
	git.Checkout = lifetime.Outlasting1(outlive, "checking out a branch", git.Checkout)
	git.CreateWorktree = lifetime.OutlastingAnswer2(outlive, "adding a worktree", git.CreateWorktree)
	git.Fetch = lifetime.Outlasting0(outlive, "fetching", git.Fetch)
	git.Commit = lifetime.OutlastingRun1(outlive, "committing", git.Commit)
	git.Push = lifetime.OutlastingRun1(outlive, "pushing", git.Push)
	git.Amend = lifetime.OutlastingRun0(outlive, "amending the last commit", git.Amend)
	git.Fixup = lifetime.OutlastingRun1(outlive, "making a fixup commit", git.Fixup)
	git.Rebase = lifetime.OutlastingRun1(outlive, "rebasing", git.Rebase)
	git.Finish = lifetime.Outlasting1(outlive, "finishing the branch", git.Finish)
	git.LinkIssue = lifetime.Outlasting2(outlive, "linking the branch to an issue", git.LinkIssue)
	git.UnlinkIssue = lifetime.Outlasting1(outlive, "unlinking the branch from its issue", git.UnlinkIssue)

	return git
}

// outlastingForge is the forge with its writes outlasting the session.
func outlastingForge(outlive lifetime.Outliving, forge seams.Forge, noun string) seams.Forge {
	forge.CreatePullRequest = lifetime.OutlastingAnswer1(outlive, "opening a "+noun, forge.CreatePullRequest)
	forge.EditPullRequest = lifetime.OutlastingAnswer2(outlive, "editing the "+noun, forge.EditPullRequest)
	forge.RewriteDescription = lifetime.OutlastingAnswer2(
		outlive, "rewriting the "+noun+"'s description", forge.RewriteDescription)
	forge.Rerun = lifetime.OutlastingAnswer2(outlive, "rerunning a check", forge.Rerun)
	forge.Merge = lifetime.Outlasting2(outlive, "merging the "+noun, forge.Merge)

	return forge
}

// outlastingStore is the store with its records outlasting the session.
func outlastingStore(outlive lifetime.Outliving, store seams.Store) seams.Store {
	store.RecordScope = lifetime.Outlasting1(outlive, "remembering the commit scope", store.RecordScope)
	store.RecordAnnounce = lifetime.Outlasting1(outlive, "remembering an announcement", store.RecordAnnounce)
	store.CacheIssues = lifetime.Outlasting2(outlive, "caching the issue list", store.CacheIssues)
	store.LinkOwner = lifetime.Outlasting2(outlive, "linking a code owner", store.LinkOwner)
	store.ForgetOwner = lifetime.Outlasting2(outlive, "forgetting a code owner's link", store.ForgetOwner)
	store.SetRepoGroups = lifetime.Outlasting2(outlive, "setting the repository's groups", store.SetRepoGroups)
	store.RecordGroups = lifetime.Outlasting2(outlive, "remembering the groups tagged", store.RecordGroups)
	store.Favor = lifetime.Outlasting1(outlive, "adding a favorite", store.Favor)
	store.Unfavor = lifetime.Outlasting1(outlive, "removing a favorite", store.Unfavor)

	return store
}

// outlastingTasks is Taskwarrior with its writes outlasting the session.
func outlastingTasks(outlive lifetime.Outliving, tasks seams.Tasks) seams.Tasks {
	tasks.Add = lifetime.OutlastingAnswer1(outlive, "adding a task", tasks.Add)
	tasks.Start = lifetime.Outlasting1(outlive, "starting a task", tasks.Start)
	tasks.Stop = lifetime.Outlasting1(outlive, "stopping a task", tasks.Stop)
	tasks.Done = lifetime.Outlasting1(outlive, "marking a task done", tasks.Done)
	tasks.Annotate = lifetime.Outlasting2(outlive, "annotating a task", tasks.Annotate)
	tasks.Modify = lifetime.Outlasting2(outlive, "modifying a task", tasks.Modify)
	tasks.Undo = lifetime.OutlastingAnswer0(outlive, "undoing a task change", tasks.Undo)
	tasks.Sync = lifetime.OutlastingAnswer0(outlive, "syncing tasks", tasks.Sync)

	return tasks
}

// outlastingSettings is the settings with saving them, and writing a first
// file, outlasting the session.
func outlastingSettings(outlive lifetime.Outliving, settings seams.Settings) seams.Settings {
	settings.Setup.Write = lifetime.OutlastingAnswer1(outlive, "writing the configuration file", settings.Setup.Write)

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
