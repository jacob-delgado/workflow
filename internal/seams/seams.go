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
// cli.WebDeps, so any surface can import them without importing the terminal.
// The package therefore imports only the domain packages and internal/loop,
// and nothing that imports a surface. A depguard rule in .golangci.yml holds
// that direction.
package seams

import (
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// Jira is what a surface asks of the issue tracker.
type Jira struct {
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

// Git is what a surface asks of the repository.
type Git struct {
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
	// Kind is the forge the remote points at, so a surface can call a change a
	// "pull request" or a "merge request".
	Kind forge.Kind
}

// Messaging is what a surface asks of the messaging service.
type Messaging struct {
	// Post sends text to a channel, or to the configured default when channel is
	// empty. A webhook ignores the channel and posts where it is bound.
	Post func(channel, text string) error
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
