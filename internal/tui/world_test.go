// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// testNow is the time the fake clock tells.
func testNow() time.Time {
	return time.Date(2026, 9, 16, 16, 0, 0, 0, time.UTC)
}

// Fixtures the scenario tests share.
const (
	baseRef      = "origin/main"
	baseName     = "main"
	reporter     = "Ana Lopez"
	keyEnter     = "enter"
	keyTab       = "tab"
	keyShiftTab  = "shift+tab"
	keyEsc       = "esc"
	keySpace     = "space"
	keyRight     = "right"
	keyCtrlO     = "ctrl+o"
	keyBackspace = "backspace"
	issueKey     = "PROJ-412"
	secondIssue  = "PROJ-388"
	issueSummary = "Fix token redaction"

	statusInProgress      = "In Progress"
	statusInReview        = "In Review"
	categoryIndeterminate = "indeterminate"
	fieldAssignee         = "Assignee"
	idAssignee            = "assignee"
	featureName           = "fix/PROJ-412-fix-token-redaction"
	pullURL               = "https://github.com/example/repo/pull/42"
	pullTitle             = "fix(config): redact tokens"
	slackChannel          = "#dev"
	shortComment          = "a short comment"
	untrackedNotes        = "notes.txt"
)

// world is everything outside the interface, faked, and a record of what the
// interface asked of it. Each field is an answer a test can change before the
// world is used.
type world struct {
	mu    sync.Mutex
	calls []string

	cfg           config.Config
	issues        []jira.Issue
	viewIssues    map[string][]jira.Issue
	pageSize      int
	detail        jira.IssueDetail
	detailErr     error
	moves         []jira.Transition
	transitionErr error
	commentErr    error
	assignErr     error
	worklogErr    error
	linkErr       error
	postedChannel string

	branch            gitrepo.Branch
	branches          []string
	branchesErr       error
	diff              []string
	diffErr           error
	noDiff            bool
	remoteBranches    []string
	remoteBranchesErr error
	noRemoteBranches  bool
	codeOwners        []string
	codeOwnersErr     error
	noCodeOwners      bool
	recentSubjects    []string
	recentSubjectsErr error
	noRecentSubjects  bool
	changes           []gitrepo.Change
	changesErr        error
	noChanges         bool
	stageErr          error
	createErr         error
	worktreeErr       error
	checkoutErr       error
	finishErr         error
	fetchErr          error
	commitLines       []string
	commitErr         error
	pushLines         []string
	pushErr           error
	amendLines        []string
	amendErr          error
	noAmend           bool
	fixupLines        []string
	fixupErr          error
	noFixup           bool
	rebaseLines       []string
	rebaseErr         error

	commitStartErr  error
	ciErr           error
	rerunErr        error
	nothingToRerun  bool
	mergeErr        error
	mergeMethods    []forge.MergeMethod
	mergeMethodsErr error
	authorErr       error

	pull        forge.PullRequest
	pullFound   bool
	pullErr     error
	openErr     error
	editPullErr error
	reviewerErr error
	reviews     []forge.ReviewRequest
	reviewsErr  error
	ci          []forge.CI
	templates   []forge.Template
	forgeKind   forge.Kind
	author      string
	postErr     error
	// postParked, when set, is told of each post once it is recorded, and the post
	// then waits for postRelease to close: a Slack slow to answer, caught with
	// the post sent and not yet answered.
	postParked  chan struct{}
	postRelease chan struct{}
	gitHooks    []hooks.GitHook
	configured  bool
	writeErr    error
	// installErr is lefthook failing to install once lefthook.yml is written.
	installErr error
	// learnedScope is the commit scope the store reports as last used here; empty
	// means nothing was recorded.
	learnedScope string
	// storedAnnounces is what the store reports being posted in earlier sessions.
	storedAnnounces []tui.AnnouncedPost
	// cachedIssues is the issue list the store reports for the active view.
	cachedIssues []jira.Issue

	edited    string
	editErr   error
	editorErr error
	// editHelp is the guidance the fake editor was last opened with, so a test
	// can assert on the note the interface chose to show.
	editHelp   string
	openURLErr error
	ciInterval time.Duration
	// unresolved are the places the fake editor cannot find a file for, the way
	// go test prints one relative to its package.
	unresolved []string
	// runBlocks makes a streamed run stream nothing and never finish until it is
	// stopped, standing in for a hook that hangs; runStop is set to a flag that
	// records whether that run's own Stop was called.
	runBlocks bool
	runStop   *bool
}

// errRunStopped is how a stopped streamed run reports that it was killed.
var errRunStopped = errors.New("the run was stopped")

// exitStatusError fakes how internal/proc reports a program that ended with a
// failure status of its own: in the status's own words, answering to
// proc.ErrExitStatus.
type exitStatusError struct {
	code int
}

// Error is the status as the process puts it.
func (e exitStatusError) Error() string {
	return "exit status " + strconv.Itoa(e.code)
}

// Is answers to proc.ErrExitStatus, as the real status does.
func (e exitStatusError) Is(target error) bool {
	return target == proc.ErrExitStatus
}

// gitExited is git ending with a failure status, shaped the way internal/proc
// reports it: "git: exit status 1".
func gitExited(code int) error {
	return fmt.Errorf("git: %w", exitStatusError{code: code})
}

// blockingOutput is a streamed program that has not finished: its lines stay
// open, and Wait reports it killed, until Stop closes them. Stop records that it
// was called, so a test can prove a stop key reached the run.
func (w *world) blockingOutput() proc.Output {
	lines := make(chan string)
	stopped := false
	w.runStop = &stopped

	stop := func() {
		if !stopped {
			stopped = true

			close(lines)
		}
	}

	return proc.Output{Lines: lines, Wait: func() error { return errRunStopped }, Stop: stop}
}

// newWorld is a repository on a feature branch for an issue in progress, with
// a pull request whose CI passed.
func newWorld() *world {
	return &world{
		cfg: completeConfig(),
		issues: []jira.Issue{
			{Key: issueKey, Summary: issueSummary, Status: "In Progress", StatusCategory: "indeterminate", Type: "Bug"},
			{Key: secondIssue, Summary: "Add retries", Status: "To Do", StatusCategory: "new", Type: "Story"},
		},
		pageSize: 50,
		detail: jira.IssueDetail{
			Issue: jira.Issue{Key: issueKey}, Reporter: reporter, Description: "Tokens reach the log.",
			Comments: []jira.Comment{
				{Author: reporter, Body: "Repro'd on 8.2.1", Created: testNow().Add(-2 * time.Hour)},
			},
			CommentTotal: 1,
		},
		branch: gitrepo.Branch{
			Name: featureName, Head: "abc123", Base: baseRef,
			Upstream: "origin/" + featureName, PushRemote: gitrepo.DefaultRemote,
			Commits: []gitrepo.Commit{{Hash: "1a2b3c4", Subject: pullTitle}},
		},
		changes:   []gitrepo.Change{{Path: redactPath, Staged: 'M', Unstaged: ' '}},
		pull:      forge.PullRequest{Number: 42, URL: pullURL, Title: pullTitle, Draft: false},
		pullFound: true,
		ci:        []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1, Failed: 0}},
		author:    "jacob",
	}
}

// record notes a call.
func (w *world) record(call string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.calls = append(w.calls, call)
}

// asked reports every call starting with prefix.
func (w *world) asked(prefix string) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var matching []string

	for _, call := range w.calls {
		if strings.HasPrefix(call, prefix) {
			matching = append(matching, call)
		}
	}

	return matching
}

// output is a program's output that is already complete. Its Stop has nothing
// left to end, but is there because proc.Start always supplies one.
func output(lines []string, err error) proc.Output {
	stream := make(chan string, len(lines))
	for _, line := range lines {
		stream <- line
	}

	close(stream)

	return proc.Output{Lines: stream, Wait: func() error { return err }, Stop: func() {}}
}

// deps wires the world to the interface.
func (w *world) deps() tui.Deps {
	return tui.Deps{
		Jira:  w.jiraDeps(),
		Git:   w.gitDeps(),
		Forge: w.forgeDeps(),
		Messaging: tui.MessagingDeps{Post: func(channel, text string) error {
			w.record("post " + text)
			w.rememberChannel(channel)

			if w.postParked != nil {
				w.postParked <- struct{}{}

				<-w.postRelease
			}

			return w.postErr
		}},
		Hooks:      w.hookDeps(),
		Editor:     w.editorDeps(),
		Store:      w.storeDeps(),
		Clock:      testNow,
		CIInterval: w.ciInterval,
		After:      fakeAfter,
		Notify:     func() { w.record("notify") },
		OpenURL: func(url string) error {
			w.record("browse " + url)

			return w.openURLErr
		},
		Copy: func(text string) tea.Cmd {
			// Record in the returned command, as tea.SetClipboard does its work
			// there, so a test that never runs the command sees no copy — a handler
			// that drops the command is caught rather than passing.
			return func() tea.Msg {
				w.record("copy " + text)

				return nil
			}
		},
	}
}

// storeDeps fakes the on-disk store: it reports learnedScope as the last one used
// here and records what a commit remembers.
func (w *world) storeDeps() tui.StoreDeps {
	return tui.StoreDeps{
		LastScope: func() (string, bool) {
			return w.learnedScope, w.learnedScope != ""
		},
		RecordScope: func(scope string) {
			w.record("scope " + scope)
		},
		Announced: func() []tui.AnnouncedPost {
			return w.storedAnnounces
		},
		RecordAnnounce: func(post tui.AnnouncedPost) {
			w.record("announce " + strconv.Itoa(post.Pull) + " " + strconv.Itoa(post.Moment))
		},
		CachedIssues: func(_ string) ([]jira.Issue, bool) {
			return w.cachedIssues, len(w.cachedIssues) > 0
		},
		CacheIssues: func(_ string, issues []jira.Issue) {
			w.record("cache " + strconv.Itoa(len(issues)))
		},
	}
}

// jiraDeps fakes Jira.
func (w *world) jiraDeps() tui.JiraDeps {
	return tui.JiraDeps{
		Search: func(jql string, startAt int) (jira.SearchResult, error) {
			w.record("search " + jql)

			issues := w.issues
			if scoped, ok := w.viewIssues[jql]; ok {
				issues = scoped
			}

			start := min(startAt, len(issues))
			end := min(start+w.pageSize, len(issues))

			return jira.SearchResult{Issues: slices.Clone(issues[start:end]), Total: len(issues)}, nil
		},
		Issue: func(key jira.Key) (jira.IssueDetail, error) {
			w.record("issue " + string(key))

			return w.detail, w.detailErr
		},
		Transitions: func(key jira.Key) ([]jira.Transition, error) {
			w.record("transitions " + string(key))

			return w.moves, nil
		},
		Transition: func(key jira.Key, to jira.Transition, values []jira.FieldValue) error {
			var call strings.Builder

			call.WriteString("transition " + string(key) + " " + to.ID)

			for _, value := range values {
				call.WriteString(" " + value.Field.ID + "=" + value.OptionID + strings.Join(value.OptionIDs, ",") + value.Text)
			}

			w.record(call.String())

			return w.transitionErr
		},
		Comment: func(key jira.Key, text string) (jira.Comment, error) {
			w.record("comment " + string(key) + " " + text)

			return jira.Comment{Author: "jacob", Body: text, Created: testNow()}, w.commentErr
		},
		Assign: func(key jira.Key, assignee string) error {
			w.record("assign " + string(key) + " " + assignee)

			return w.assignErr
		},
		AddWorklog: func(key jira.Key, timeSpent, comment string) (jira.Worklog, error) {
			w.record("worklog " + string(key) + " " + timeSpent + " " + comment)

			return jira.Worklog{ID: "1", TimeSpent: timeSpent}, w.worklogErr
		},
		LinkPullRequest: func(key jira.Key, pullURL, title string) error {
			w.record("link " + string(key) + " " + pullURL + " " + title)

			return w.linkErr
		},
		BrowseURL: func(key jira.Key) string { return "https://jira.example.com/browse/" + string(key) },
	}
}

// hookDeps fakes lefthook.
func (w *world) hookDeps() tui.HookDeps {
	return tui.HookDeps{
		Run: func(hook string) (proc.Output, error) {
			w.record("hook " + hook)

			if w.runBlocks {
				return w.blockingOutput(), nil
			}

			return output(w.commitLines, w.commitErr), nil
		},
		Existing: func() ([]hooks.GitHook, bool) { return w.gitHooks, w.configured },
		Write: func(generated hooks.Generated) error {
			w.record("write " + generated.Config)

			if w.writeErr != nil {
				return w.writeErr
			}

			w.configured = true

			return w.installErr
		},
	}
}

// editorDeps fakes the editor: whatever is edited comes back as edited.
func (w *world) editorDeps() tui.EditorDeps {
	return tui.EditorDeps{
		Edit: func(text, help string, done func(string, error) tea.Msg) tea.Cmd {
			w.record("edit " + text)
			w.editHelp = help

			return func() tea.Msg { return done(w.edited, w.editErr) }
		},
		Open: func(file string, line int, done func(error) tea.Msg) tea.Cmd {
			w.record("open-editor " + file + ":" + strconv.Itoa(line))

			return func() tea.Msg { return done(w.editorErr) }
		},
		Resolve: func(places []string) map[string]string {
			w.record("resolve " + strings.Join(places, " "))

			resolved := make(map[string]string, len(places))

			for _, place := range places {
				if !slices.Contains(w.unresolved, place) {
					resolved[place] = place
				}
			}

			return resolved
		},
	}
}
