// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Deps is everything the interface asks of the world outside the terminal,
// grouped by what it asks. Each is a function rather than a client, so the model
// never holds a context or a credential, and a test can hand it canned answers
// without a network, a repository or a subprocess.
//
// A load the interface starts by itself is skipped when its function is nil, so
// a test need only supply what it exercises. An action is only offered once the
// model has what it acts on.
type Deps struct {
	Jira      JiraDeps
	Git       GitDeps
	Forge     ForgeDeps
	Messaging MessagingDeps
	Hooks     HookDeps
	Editor    EditorDeps
	Store     StoreDeps
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
	OpenURL func(url string) error
	// Copy puts text on the system clipboard. In production it is tea.SetClipboard,
	// which writes it through the terminal's own OSC 52 sequence — no program, and
	// it works over SSH. Nil where the interface cannot reach the terminal.
	Copy func(text string) tea.Cmd
}

// JiraDeps is what the interface asks of Jira.
type JiraDeps struct {
	Search      func(jql string, startAt int) (jira.SearchResult, error)
	Issue       func(issueKey jira.Key) (jira.IssueDetail, error)
	Transitions func(issueKey jira.Key) ([]jira.Transition, error)
	Transition  func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	Comment     func(issueKey jira.Key, text string) (jira.Comment, error)
	// Assign sets an issue's assignee by username. Nil when Jira is not
	// configured.
	Assign func(issueKey jira.Key, assignee string) error
	// AddWorklog logs work against an issue: a duration and an optional note.
	AddWorklog func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error)
	// LinkPullRequest records a pull request as a web link on an issue. Nil when
	// Jira is not configured.
	LinkPullRequest func(issueKey jira.Key, pullURL, title string) error
	// BrowseURL links an issue for someone to click.
	BrowseURL func(issueKey jira.Key) string
}

// GitDeps is what the interface asks of the repository.
type GitDeps struct {
	Branch  func() (gitrepo.Branch, error)
	Changes func() ([]gitrepo.Change, error)
	// Diff reads a changed file's diff against HEAD, line by line, so it can be
	// read before staging. Nil when there is no repository.
	Diff         func(change gitrepo.Change) ([]string, error)
	Stage        func(change gitrepo.Change) error
	Unstage      func(change gitrepo.Change) error
	CreateBranch func(name, start string) error
	// Branches lists the local branches; Checkout switches to one. Nil when
	// there is no repository.
	Branches func() ([]string, error)
	Checkout func(name string) error
	// RemoteBranches lists the branches on the remotes by name, so the pull
	// request's base field can complete to one. Nil when there is no repository.
	RemoteBranches func() ([]string, error)
	// CodeOwners reads the user handles the CODEOWNERS file names, so the pull
	// request's reviewers field can suggest them. Nil when there is no repository.
	CodeOwners func() ([]string, error)
	// RecentSubjects reads the subjects of recent commits, so the commit scope
	// field can complete from the scopes already in use. Nil when there is no
	// repository.
	RecentSubjects func() ([]string, error)
	// CreateWorktree creates a branch in a new worktree beside the repository and
	// returns where it put it, so a task can be started without disturbing the
	// current checkout. Nil when there is no repository.
	CreateWorktree func(name, start string) (string, error)
	// Fetch updates origin's tracking refs, so a new branch starts from what
	// origin holds now. Nil when there is no repository.
	Fetch func() error
	// Commit and Push stream their output, because hooks run inside both.
	Commit func(message string) (proc.Output, error)
	Push   func(branch string) (proc.Output, error)
	// Amend folds the staged changes into the last commit, keeping its message;
	// Fixup records a fixup! of an earlier commit. Both stream their output,
	// because hooks run inside them. Nil when there is no repository.
	Amend func() (proc.Output, error)
	Fixup func(hash string) (proc.Output, error)
	// Rebase replays the branch onto base and streams its output. A conflict
	// leaves the repository mid-rebase for the shell. Nil when there is no
	// repository.
	Rebase func(base string) (proc.Output, error)
	// Finish finishes a merged branch: switch to base, fast-forward it, delete
	// the branch. Nil when there is no repository.
	Finish func(branch, base string) error
}

// ForgeDeps is what the interface asks of GitHub or GitLab.
type ForgeDeps struct {
	FindPullRequest   func(branch string) (forge.PullRequest, bool, error)
	CreatePullRequest func(request forge.NewPullRequest) (forge.PullRequest, error)
	EditPullRequest   func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error)
	CheckStatus       func(pull forge.PullRequest, head string) (forge.CI, error)
	// Rerun re-runs the failed CI on a pull request and reports whether anything
	// was re-run. It needs a write scope the read path does not, so it can be
	// refused where CheckStatus was not. Nil when there is no forge.
	Rerun func(pull forge.PullRequest, head string) (bool, error)
	// Merge merges a pull request by a method the repository permits, and
	// MergeMethods lists those methods. Merge needs a write scope the read path
	// does not. Both nil when there is no forge.
	Merge        func(pull forge.PullRequest, method forge.MergeMethod) error
	MergeMethods func() ([]forge.MergeMethod, error)
	// ReviewRequests lists the pull requests on the forge that ask for your
	// review, across whichever repositories requested you.
	ReviewRequests func() ([]forge.ReviewRequest, error)
	Templates      func() []forge.Template
	// Author is who the forge credential belongs to, to say who opened a pull
	// request.
	Author func() (string, error)
	// Kind is the forge the remote points at, so the interface can call a change
	// a "pull request" or a "merge request".
	Kind forge.Kind
}

// MessagingDeps is what the interface asks of the messaging service.
type MessagingDeps struct {
	// Post sends text to a channel, or to the configured default when channel is
	// empty. A webhook ignores the channel and posts where it is bound.
	Post func(channel, text string) error
}

// StoreDeps is what the interface asks of the on-disk store, bound to this
// repository. Nil functions mean no store — a disabled one, nowhere to keep it,
// or a dry run — so the interface simply learns nothing.
type StoreDeps struct {
	// LastScope is the commit scope last used in this repository, if one was, so
	// the composer can open on it.
	LastScope func() (string, bool)
	// RecordScope remembers the commit scope just used in this repository.
	RecordScope func(scope string)
	// Announced is every pull request announced in this repository in an earlier
	// session, so the messaging pane opens knowing what has already been posted.
	Announced func() []AnnouncedPost
	// RecordAnnounce remembers that a pull request was just announced at a moment.
	RecordAnnounce func(post AnnouncedPost)
	// CachedIssues is the issue list last seen for a view, so the pane can show it
	// at once before the tracker answers.
	CachedIssues func(view string) ([]jira.Issue, bool)
	// CacheIssues remembers the issue list just seen for a view.
	CacheIssues func(view string, issues []jira.Issue)
}

// AnnouncedPost is one announcement the store remembers: which pull request, and
// the moment it marked.
type AnnouncedPost struct {
	Pull   int
	Moment int
}

// HookDeps is what the interface asks of lefthook.
type HookDeps struct {
	// Run runs one hook, streaming its output.
	Run func(hook string) (proc.Output, error)
	// Existing reports the hooks git would run that lefthook does not manage,
	// and whether the repository already configures lefthook.
	Existing func() ([]hooks.GitHook, bool)
	// Write creates a generated configuration and installs lefthook's hooks.
	Write func(generated hooks.Generated) error
}

// EditorDeps hands text and files to the user's editor, which takes the
// terminal while it is open.
type EditorDeps struct {
	Edit func(text, help string, done func(string, error) tea.Msg) tea.Cmd
	Open func(file string, line int, done func(error) tea.Msg) tea.Cmd
	// Resolve turns a place a tool printed into the file that opens it, or
	// reports that none does. A place relative to a package, not the root, is the
	// common miss.
	Resolve func(file string) (string, bool)
}

// resolvedFailures keeps only the places that resolve to a file, rewriting each
// to the path that opens it, so a place a tool printed relative to its package
// is either found below the root or dropped rather than offered as a jump that
// opens nothing. With no Resolve seam the places are left as they came.
func (d Deps) resolvedFailures(found []hooks.Location) []hooks.Location {
	if d.Editor.Resolve == nil {
		return found
	}

	kept := make([]hooks.Location, 0, len(found))

	for _, place := range found {
		file, ok := d.Editor.Resolve(place.File)
		if ok {
			place.File = file
			kept = append(kept, place)
		}
	}

	return kept
}

// defaultCIInterval is how often CI is asked about when nothing says otherwise:
// often enough to post soon after a pass, rarely enough not to spend a forge's
// rate limit on it.
const defaultCIInterval = 20 * time.Second

// ciInterval is how often CI is asked about.
func (d Deps) ciInterval() time.Duration {
	if d.CIInterval <= 0 {
		return defaultCIInterval
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
