// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package seams declares what a surface asks of each system outside the
// process, one bundle per system. Each is a struct of plain functions over the
// domain packages' own types, and loop's record of an announcement
// (loop.Announced), so a surface never holds a context or a credential, and a
// test can hand it canned answers without a network, a repository or a
// subprocess.
//
// The command line and the terminal interface take these bundles, and the web
// server takes the same functions narrowed into a webserver.Deps by
// cli.WebDeps — Taskwarrior's bundle whole, since the API uses every function
// in it — so any surface can import them without importing the terminal.
// The package therefore imports only the domain packages and internal/loop,
// and nothing that imports a surface. A depguard rule in .golangci.yml holds
// that direction.
package seams

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/codeowners"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// Jira is what a surface asks of the issue tracker.
type Jira struct {
	Search func(jql string, startAt int) (jira.SearchResult, error)
	// SearchLenient is Search for a query naming issue keys the tracker may not
	// know — a branch's issue since deleted or hidden — which it skips rather
	// than refusing the whole query.
	SearchLenient func(jql string, startAt int) (jira.SearchResult, error)
	Issue         func(issueKey jira.Key) (jira.IssueDetail, error)
	Transitions   func(issueKey jira.Key) ([]jira.Transition, error)
	Transition    func(issueKey jira.Key, to jira.Transition, values []jira.FieldValue) error
	// Comment posts text on an issue, in Jira or on the forge as its key says,
	// exactly as given: what a surface converts it to first is
	// loop.CommentMarkupOf's to decide.
	Comment func(issueKey jira.Key, text string) (jira.Comment, error)
	// Assign sets an issue's assignee by username, in Jira or on the forge as
	// its key says.
	Assign func(issueKey jira.Key, assignee string) error
	// AddWorklog logs work against an issue: a duration and an optional note.
	AddWorklog func(issueKey jira.Key, timeSpent, comment string) (jira.Worklog, error)
	// LinkPullRequest records a pull request as a web link on an issue. Nil when
	// Jira is not configured.
	LinkPullRequest func(issueKey jira.Key, pullURL, title string) error
	// BrowseURL links an issue for someone to click.
	BrowseURL func(issueKey jira.Key) string
	// Activity is what you did to Jira issues from start up to end, for the
	// Summary. Nil without Jira: a forge issue's is the Forge seam's.
	Activity func(start, end time.Time) (jira.Activity, error)
}

// Git is what a surface asks of the repository.
type Git struct {
	Branch  func() (gitrepo.Branch, error)
	Changes func() ([]gitrepo.Change, error)
	// CommitsBetween is the commits you wrote from start up to end, for the
	// Summary: in the repository workflow is in and in every favorite that is
	// a repository, each read once.
	CommitsBetween func(start, end time.Time) []loop.RepositoryCommits
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
	// ChangedPaths lists the paths the branch changes since it left base, and
	// CodeOwnersAt reads the CODEOWNERS file as base holds it, in the forge's
	// dialect, reporting false when there is none: together they name the
	// owners of a pull request's changes. Nil when there is no repository.
	ChangedPaths func(base string) ([]string, error)
	CodeOwnersAt func(base string) (codeowners.File, bool, error)
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
	// IssueLinks is every branch linked to an issue by hand, by branch name;
	// LinkIssue links a branch to an issue, and UnlinkIssue forgets its link.
	// Nil when there is no repository.
	IssueLinks  func() map[string]string
	LinkIssue   func(branch, issueKey string) error
	UnlinkIssue func(branch string) error
}

// Forge is what a surface asks of GitHub or GitLab.
type Forge struct {
	FindPullRequest   func(branch string) (forge.PullRequest, bool, error)
	CreatePullRequest func(request forge.NewPullRequest) (forge.PullRequest, error)
	EditPullRequest   func(pull forge.PullRequest, edit forge.PullRequestEdit) (forge.PullRequest, error)
	CheckStatus       func(pull forge.PullRequest, head string) (forge.CI, error)
	// Rerun re-runs the failed CI on a pull request and reports whether anything
	// was re-run. It needs a write scope the read path does not, so it can be
	// refused where CheckStatus was not. Nil when there is no forge.
	Rerun func(pull forge.PullRequest, head string) (bool, error)
	// JobLog reads the end of a failed check's log, for one the forge keeps a
	// log for. Nil when there is no forge.
	JobLog func(check forge.Check) (forge.JobLog, error)
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
	// GroupMembers lists a GitLab group's direct members who can review, by
	// username, the group named by its full path, so a CODEOWNERS group can
	// stand for its people. Nil on GitHub, whose teams review as teams, and
	// with no forge.
	GroupMembers func(group string) ([]string, error)
	// IsGroup reports whether a bare CODEOWNERS name, @acme, is a top-level
	// GitLab group rather than a user, so it is tagged as a team. Nil on
	// GitHub, whose teams are spelled org/team, and with no forge.
	IsGroup func(name string) (bool, error)
	// Kind is the forge the remote points at, so a surface can call a change a
	// "pull request" or a "merge request".
	Kind forge.Kind
	// Activity is what you did on the forge from start up to end, in any
	// repository, for the Summary.
	Activity func(start, end time.Time) (forge.Activity, error)
}

// Messaging is what a surface asks of the messaging service.
type Messaging struct {
	// Post sends text to a channel, or to the configured default when channel is
	// empty. A webhook ignores the channel and posts where it is bound.
	Post func(channel, text string) error
	// ChannelMembers is everyone in a channel, named as configured — by name or
	// ID — labeled by the name Slack shows for them. The directory reads are
	// bound whatever the settings, since Settings can switch to or from a Slack
	// user token while workflow runs: a read answering
	// messaging.ErrNoCredential means tagging is unavailable under the settings
	// in effect, not that it failed. A nil read means none can be made.
	ChannelMembers func(channel string) ([]loop.SlackTarget, error)
	// UserGroups is every user group in the Slack workspace.
	UserGroups func() ([]loop.SlackTarget, error)
	// RefreshDirectory drops the directory read so far this session, so the
	// next read asks Slack again.
	RefreshDirectory func()
	// Workspace is the ID of the Slack workspace the user token is for, held
	// with the directory's reads. The store's links are kept per workspace,
	// so a surface reads it before them. It answers messaging.ErrNoCredential
	// as the directory does, and an error when Slack cannot say, when a
	// surface tags no one and says why.
	Workspace func() (string, error)
	// Grant is the scopes Slack lists the user token as granted, held with
	// Workspace, so a missing scope tagging needs is told without reading the
	// directory. A grant Slack did not list lacks none.
	Grant func() (messaging.Grant, error)
}

// Store is what a surface asks of the on-disk store, bound to this repository.
// The wiring binds every function. A disabled store, or one with nowhere to
// keep its file, no-ops through them, and a command's --dry-run reads a store
// already on disk and writes nothing. Nil functions mean the surface was given
// no store at all, as the terminal is under a dry run, and it simply learns
// nothing. Under a dry run, --web is handed the bound functions and calls none.
type Store struct {
	// LastScope is the commit scope last used in this repository, if one was, so
	// the composer can open on it.
	LastScope func() (string, bool)
	// RecordScope remembers the commit scope just used in this repository.
	RecordScope func(scope string)
	// Announced is every pull request announced in this repository in an earlier
	// session, so a surface starts knowing what has already been posted.
	Announced func() []loop.Announced
	// RecordAnnounce remembers that a pull request was just announced at a moment.
	RecordAnnounce func(made loop.Announced)
	// CachedIssues is the issue list last seen for a view, so it can be shown at
	// once before the tracker answers.
	CachedIssues func(view string) ([]jira.Issue, bool)
	// CacheIssues remembers the issue list just seen for a view.
	CacheIssues func(view string, issues []jira.Issue)
	// OwnerLinks is every forge owner decided on this repository's forge host
	// as a Slack workspace, Messaging.Workspace, sees them: whom each is on
	// Slack there, or that they are not on Slack there. An owner decided only
	// in other workspaces is left out, so they are asked again. Kept data
	// survives the cache's schema changes; a file at another kept schema
	// version reads as none. The kept seams, OwnerLinks through RecordGroups,
	// are nil when the store keeps nothing, and the owner ones when there is
	// no forge host to key them by. Each refuses an empty workspace with
	// store.ErrNoWorkspace.
	OwnerLinks func(workspace string) ([]loop.OwnerLink, error)
	// LinkOwner records what was decided for a forge owner on this forge
	// host, and whether they are a person or a team: whom they are on Slack
	// in a workspace — a user for a person, a user group for a team — or that
	// they are not on Slack there, so they are not asked again there.
	LinkOwner func(workspace string, decision loop.OwnerLink) error
	// ForgetOwner drops what was decided for a forge owner in a workspace, so
	// they are asked again there.
	ForgetOwner func(workspace, owner string) error
	// RepoGroups is the user groups of a workspace this repository may tag.
	RepoGroups func(workspace string) ([]loop.SlackTarget, error)
	// SetRepoGroups replaces the user groups of a workspace this repository
	// may tag.
	SetRepoGroups func(workspace string, groups []loop.SlackTarget) error
	// LastGroups is the group IDs of a workspace last chosen for this
	// repository's announcement, and whether a choice was recorded at all.
	LastGroups func(workspace string) ([]string, bool)
	// RecordGroups remembers the groups of a workspace just chosen; one not
	// among RepoGroups, such as a group linked to an owning team, is left out
	// of the choice.
	RecordGroups func(workspace string, ids []string) error
	// Favorites is the directories you marked, by path. It, Favor and Unfavor
	// are nil when the store keeps nothing; a dry run keeps Favorites alone.
	Favorites func() ([]string, error)
	// Favor marks a directory, an absolute path, a favorite.
	Favor func(dir string) error
	// Unfavor forgets a directory as a favorite.
	Unfavor func(dir string) error
}

// Place is a directory a surface can work in, as it is now: the directory,
// the root of the repository it is in, origin's host and path, and the
// configuration files that apply there.
type Place struct {
	Dir string
	// Root is the repository's root, or "" outside a repository.
	Root string
	// Remote is origin's host and path, or zero when origin is missing or
	// names no repository on a forge.
	Remote forge.Repo
	Config config.Files
}

// Repositories is what a surface asks of the directories it can work in.
type Repositories struct {
	// Here is where this session works.
	Here Place
	// Home is your home directory, or "" when there is none, which a
	// directory under it is written from.
	Home string
	// Look reads a directory as a Place, refusing one that is not an absolute
	// path to a directory that is there with workdirs' sentinels.
	Look func(dir string) (Place, error)
	// Subdirectories is the directories in one whose names start with a
	// prefix, all of them for "".
	Subdirectories func(dir, prefix string) (workdirs.Listing, error)
	// Worktrees is the working trees of the repository this session works
	// in, the main one first, read each time it is asked. Nil outside a
	// repository.
	Worktrees func() ([]gitrepo.Worktree, error)
}

// Hooks is what a surface asks of lefthook.
type Hooks struct {
	// Run runs one hook, streaming its output.
	Run func(hook string) (proc.Output, error)
	// Existing reports the hooks git would run that lefthook does not manage,
	// and whether the repository already configures lefthook.
	Existing func() ([]hooks.GitHook, bool)
	// Write creates a generated configuration and installs lefthook's hooks.
	Write func(generated hooks.Generated) error
}

// Tasks is what a surface asks of Taskwarrior. Every function is nil when no
// task program is on PATH and none is configured, or taskwarrior.disabled is
// set, so the Tasks pane and section say so and offer nothing. Anything else is
// settled on first use: a configured program that is not there, one that
// answers as go-task, an older Taskwarrior, one never run, or one whose taskrc
// has a malformed line makes Install and every other function return
// taskwarrior.ErrNotInstalled, ErrNotTaskwarrior, ErrTooOld, ErrNotConfigured
// or ErrRefused, which the surfaces word.
type Tasks struct {
	// Install is the Taskwarrior that answered.
	Install func() (taskwarrior.Install, error)
	// Pending is the pending tasks of the active context, most urgent first.
	Pending func() (taskwarrior.List, error)
	// Touched is every task of the active context that changed since a time,
	// whatever its status but deleted, for the Summary.
	Touched func(since time.Time) ([]taskwarrior.Task, error)
	// Linked is every task linked to an issue and not deleted, whatever the
	// active context hides.
	Linked func() ([]taskwarrior.Task, error)
	// Add creates a task from a line in Taskwarrior's grammar and returns its
	// uuid.
	Add func(line string) (string, error)
	// Start, Stop and Done change the task a uuid names.
	Start func(uuid string) error
	Stop  func(uuid string) error
	Done  func(uuid string) error
	// Annotate adds text to a task as an annotation.
	Annotate func(uuid, text string) error
	// Modify changes a task by a line in Taskwarrior's grammar.
	Modify func(uuid, line string) error
	// Undo reverts Taskwarrior's last change and says how much it reverted.
	Undo func() (string, error)
	// Sync syncs with the backend the taskrc names, and says what it printed.
	Sync func() (string, error)
}
