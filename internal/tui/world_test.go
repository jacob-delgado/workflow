// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
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

// patience is how long a command may take before a test stops waiting for it.
// Everything these tests fake answers at once; what does not is a timer — the
// wait between CI checks — that a test only cares about when it shortens it.
//
// This wall-clock cap is deliberate, not a step toward Bubble Tea quiescence.
// Settling on quiescence instead would have to tell a scheduled timer (which
// reschedules itself forever) from real work, which the framework gives no way
// to introspect; the cap sidesteps that by dropping whatever overruns. The known
// cost is that a genuinely slow command under a loaded -race runner can be
// dropped too — raise patience there rather than reworking the drain.
const patience = 400 * time.Millisecond

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
	categoryIndeterminate = "indeterminate"
	fieldAssignee         = "Assignee"
	idAssignee            = "assignee"
	featureName           = "fix/PROJ-412-fix-token-redaction"
	pullURL               = "https://github.com/example/repo/pull/42"
	pullTitle             = "fix(config): redact tokens"
	slackChannel          = "#dev"
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

	branch      gitrepo.Branch
	branches    []string
	branchesErr error
	diff        []string
	diffErr     error
	noDiff      bool
	// diffGate, when set, holds every diff read until it is closed, so a test can
	// see the pane while the diff is still being read.
	diffGate          chan struct{}
	remoteBranches    []string
	remoteBranchesErr error
	noRemoteBranches  bool
	recentSubjects    []string
	recentSubjectsErr error
	noRecentSubjects  bool
	changes           []gitrepo.Change
	stageErr          error
	createErr         error
	worktreeErr       error
	checkoutErr       error
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

	commitStartErr error
	ciErr          error
	rerunErr       error
	nothingToRerun bool
	authorErr      error

	pull       forge.PullRequest
	pullFound  bool
	pullErr    error
	openErr    error
	reviews    []forge.ReviewRequest
	reviewsErr error
	ci         []forge.CI
	templates  []forge.Template
	forgeKind  forge.Kind
	author     string
	postErr    error
	// postGate, when set, holds every post, already recorded, until it is
	// closed: a Slack that is slow to answer.
	postGate   chan struct{}
	gitHooks   []hooks.GitHook
	configured bool
	writeErr   error

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
			Name: featureName, Head: "abc123", Upstream: "origin/" + featureName, Base: baseRef,
			Commits: []gitrepo.Commit{{Hash: "1a2b3c4", Subject: pullTitle}},
		},
		changes:   []gitrepo.Change{{Path: "internal/config/redact.go", Staged: 'M', Unstaged: ' '}},
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

// nextCI is the next CI answer: each check takes the next, and the last one
// repeats.
func (w *world) nextCI() forge.CI {
	w.mu.Lock()
	defer w.mu.Unlock()

	answer := w.ci[0]
	if len(w.ci) > 1 {
		w.ci = w.ci[1:]
	}

	return answer
}

// output is a program's output that is already complete.
func output(lines []string, err error) proc.Output {
	stream := make(chan string, len(lines))
	for _, line := range lines {
		stream <- line
	}

	close(stream)

	return proc.Output{Lines: stream, Wait: func() error { return err }}
}

// deps wires the world to the interface.
func (w *world) deps() tui.Deps {
	return tui.Deps{
		Jira:  w.jiraDeps(),
		Git:   w.gitDeps(),
		Forge: w.forgeDeps(),
		Slack: tui.SlackDeps{Post: func(channel, text string) error {
			w.record("post " + text)
			w.rememberChannel(channel)

			if w.postGate != nil {
				<-w.postGate
			}

			return w.postErr
		}},
		Hooks:      w.hookDeps(),
		Editor:     w.editorDeps(),
		Clock:      testNow,
		CIInterval: w.ciInterval,
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

// forgeDeps fakes the forge.
func (w *world) forgeDeps() tui.ForgeDeps {
	return tui.ForgeDeps{
		FindPullRequest: func(branch string) (forge.PullRequest, bool, error) {
			w.record("find " + branch)

			return w.pull, w.pullFound, w.pullErr
		},
		CreatePullRequest: func(request forge.NewPullRequest) (forge.PullRequest, error) {
			w.record("open " + request.Title + " " + request.Head + ">" + request.Base + " draft=" +
				map[bool]string{false: "no", true: "yes"}[request.Draft] + "\n" + request.Body)

			return w.pull, w.openErr
		},
		CheckStatus: func(_ forge.PullRequest, head string) (forge.CI, error) {
			w.record("ci " + head)

			return w.nextCI(), w.ciErr
		},
		Rerun: func(_ forge.PullRequest, head string) (bool, error) {
			w.record("rerun " + head)

			return !w.nothingToRerun, w.rerunErr
		},
		ReviewRequests: func() ([]forge.ReviewRequest, error) {
			w.record("reviews")

			return w.reviews, w.reviewsErr
		},
		Templates: func() []forge.Template { return w.templates },
		Author:    func() (string, error) { return w.author, w.authorErr },
		Kind:      w.forgeKind,
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

			return w.writeErr
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
		Resolve: func(file string) (string, bool) {
			if slices.Contains(w.unresolved, file) {
				return "", false
			}

			return file, true
		},
	}
}
