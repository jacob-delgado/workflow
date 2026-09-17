// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	Jira   JiraDeps
	Git    GitDeps
	Forge  ForgeDeps
	Slack  SlackDeps
	Hooks  HookDeps
	Editor EditorDeps
	// Clock tells the time, for how long ago a comment was written. Nil means
	// time.Now.
	Clock func() time.Time
	// CIInterval is how often CI is asked about while it runs, or while a post
	// waits for it to pass. Zero means every twenty seconds.
	CIInterval time.Duration
}

// JiraDeps is what the interface asks of Jira.
type JiraDeps struct {
	Search      func(startAt int) (jira.SearchResult, error)
	Issue       func(issueKey string) (jira.IssueDetail, error)
	Transitions func(issueKey string) ([]jira.Transition, error)
	Transition  func(issueKey string, to jira.Transition, values []jira.FieldValue) error
	Comment     func(issueKey, text string) (jira.Comment, error)
	// LinkPullRequest records a pull request as a web link on an issue. Nil when
	// Jira is not configured.
	LinkPullRequest func(issueKey, pullURL, title string) error
	// BrowseURL links an issue for someone to click.
	BrowseURL func(issueKey string) string
}

// GitDeps is what the interface asks of the repository.
type GitDeps struct {
	Branch       func() (gitrepo.Branch, error)
	Changes      func() ([]gitrepo.Change, error)
	Stage        func(change gitrepo.Change) error
	Unstage      func(change gitrepo.Change) error
	CreateBranch func(name, start string) error
	// Branches lists the local branches; Checkout switches to one. Nil when
	// there is no repository.
	Branches func() ([]string, error)
	Checkout func(name string) error
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
}

// ForgeDeps is what the interface asks of GitHub or GitLab.
type ForgeDeps struct {
	FindPullRequest   func(branch string) (forge.PullRequest, bool, error)
	CreatePullRequest func(request forge.NewPullRequest) (forge.PullRequest, error)
	CheckStatus       func(pull forge.PullRequest, head string) (forge.CI, error)
	Templates         func() []forge.Template
	// Author is who the forge credential belongs to, to say who opened a pull
	// request.
	Author func() (string, error)
	// Kind is the forge the remote points at, so the interface can call a change
	// a "pull request" or a "merge request".
	Kind forge.Kind
}

// SlackDeps is what the interface asks of Slack.
type SlackDeps struct {
	// Post sends text to a channel, or to the configured default when channel is
	// empty. A webhook ignores the channel and posts where it is bound.
	Post func(channel, text string) error
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
