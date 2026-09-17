// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errAlreadyOpen stands in for GitHub refusing a duplicate pull request.
var errAlreadyOpen = errors.New("the forge rejected the request: A pull request already exists")

// withoutPull is the world before a pull request is opened: pushed, with a
// commit, and none found. Opening one gets the world's pull request.
func withoutPull() *world {
	w := newWorld()
	w.pullFound = false

	return w
}

func TestTheReviewPaneShowsThePullRequestAndItsCI(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ci   forge.CI
		want string
	}{
		"passed":  {ci: forge.CI{State: forge.CIPassed, Total: 3, Done: 3}, want: "● passed (3 of 3 finished)"},
		"running": {ci: forge.CI{State: forge.CIRunning, Total: 3, Done: 1}, want: "◐ running (1 of 3 finished)"},
		"failed":  {ci: forge.CI{State: forge.CIFailed, Total: 3, Done: 3, Failed: 1}, want: "✗ failed (3 of 3 finished)"},
		"none":    {ci: forge.CI{State: forge.CINone}, want: "· no checks reported"},
		// GitLab reports a pipeline as a whole, with no counts.
		"a pipeline": {ci: forge.CI{State: forge.CIPassed}, want: "● passed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reviewing := newWorld()
			reviewing.ci = []forge.CI{tt.ci}

			// Act
			view := typing(t, reviewing.live(t, 120, 40), "4").View()

			// Assert
			requireScreen(t, view, "#42 "+pullTitle, pullURL, "CI     "+tt.want)
		})
	}
}

func TestTheReviewPaneLooksForNothingOnTheBaseBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	onMain := newWorld()
	onMain.branch = gitrepo.Branch{Name: baseName, Base: baseRef}

	// Act
	view := typing(t, onMain.live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "on no feature branch")

	if calls := onMain.asked("find"); len(calls) != 0 {
		t.Errorf("looked for a pull request from the base branch: %q", calls)
	}
}

func TestTheReviewPaneOffersToOpenAPullRequest(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withoutPull().live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "no pull request yet", "n opens one from this branch's commits")
	requireScreen(t, footerLine(view), "n open pull request")
}

func TestTheReviewPaneSaysWhenTheForgeDoesNotAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	unanswered := withoutPull()
	unanswered.pullErr = errUnreachable

	// Act
	view := typing(t, unanswered.live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "✗ the forge did not answer", "✗ could not reach the forge")
}

func TestTheReviewPaneMarksADraft(t *testing.T) {
	t.Parallel()

	// Arrange
	draft := newWorld()
	draft.pull.Draft = true

	// Act
	view := typing(t, draft.live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "draft")
}

func TestNOpensAComposerStartedFromTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.templates = []forge.Template{
		{Name: "PULL_REQUEST_TEMPLATE", Body: "## What this changes\n"},
		{Name: "bugfix", Body: "## Bug\n"},
	}
	opening.edited = "Rewritten."
	model := opening.live(t, 120, 40)

	// Act: open the composer
	composer := typing(t, model, "4", "n")

	// Assert: it starts from the branch, the first template and the issue
	requireScreen(t, composer.View(), "┏━ Open pull request", "title  > "+pullTitle, "base   > main",
		"head   "+featureName, "template PULL_REQUEST_TEMPLATE (1 of 2)", "[ ] draft", "## What this changes",
		"Jira: [PROJ-412](https://jira.example.com/browse/PROJ-412)")

	// Act: take the other template, as a draft
	changed := typing(t, composer, "ctrl+t", "ctrl+r")

	// Assert: both show
	requireScreen(t, changed.View(), "template bugfix (2 of 2)", "## Bug", "[x] draft")

	// Act: write the body in the editor
	edited := typing(t, changed, "ctrl+o")

	// Assert: the body is the editor's
	requireScreen(t, edited.View(), "Rewritten.")

	// Act: open it
	opened := typing(t, edited, keyEnter)

	// Assert: the forge was asked for exactly that
	requireScreen(t, opened.View(), "● opened #42 "+pullURL)

	want := "open " + pullTitle + " " + featureName + ">main draft=yes\nRewritten."
	if calls := opening.asked("open "); len(calls) != 1 || calls[0] != want {
		t.Errorf("open calls = %q, want %q", calls, want)
	}
}

func TestTheComposerTitleAndBaseCanBeEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.edited = "Body."
	model := opening.live(t, 120, 40)

	keys := append([]string{"4", "n", "ctrl+o"}, letters("!")...)
	keys = append(append(keys, keyTab, "backspace", "backspace", "backspace", "backspace"), letters("develop")...)

	// Act: change the title and the base
	composed := typing(t, model, keys...)

	// Assert: both show changed
	requireScreen(t, composed.View(), "title  > "+pullTitle+"!", "base   > develop", "no template in this repository")

	// Act: open it
	typing(t, composed, keyEnter)

	// Assert: the forge was asked with both changes
	want := "open " + pullTitle + "! " + featureName + ">develop draft=no\nBody."
	if calls := opening.asked("open "); len(calls) != 1 || calls[0] != want {
		t.Errorf("open calls = %q, want %q", calls, want)
	}
}

func TestAPullRequestNeedsATitleAndABase(t *testing.T) {
	t.Parallel()

	erase := slices.Repeat([]string{"backspace"}, 60)

	cases := map[string]struct {
		keys []string
		want string
	}{
		"no title": {keys: append(slices.Clone(erase), keyEnter), want: "✗ a pull request needs a title"},
		"no base": {
			keys: append(append([]string{keyTab}, erase...), keyEnter),
			want: "✗ a pull request needs a base branch to merge into",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			opening := withoutPull()
			composer := typing(t, opening.live(t, 120, 40), "4", "n")

			// Act
			view := typing(t, composer, tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)

			if calls := opening.asked("open "); len(calls) != 0 {
				t.Errorf("opened a pull request with a part missing: %q", calls)
			}
		})
	}
}

func TestAnUnpushedBranchIsPushedBeforeThePullRequestOpens(t *testing.T) {
	t.Parallel()

	// Arrange
	unpushed := withoutPull()
	unpushed.branch.Upstream = ""

	// Act
	opened := typing(t, unpushed.live(t, 120, 40), "4", "n", keyEnter)

	// Assert
	requireScreen(t, opened.View(), "● opened #42")

	calls := unpushed.asked("")
	pushed := slices.Index(calls, "push "+featureName)
	opens := slices.IndexFunc(calls, func(call string) bool { return strings.HasPrefix(call, "open ") })

	if pushed < 0 || opens < pushed || len(unpushed.asked("open ")) != 1 {
		t.Errorf("calls = %q, want one push and then one pull request opened", calls)
	}
}

func TestAFailedPushOpensNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withoutPull()
	failing.branch.Upstream = ""
	failing.pushErr = errPushDenied

	// Act
	failed := typing(t, failing.live(t, 120, 40), "4", "n", keyEnter)

	// Assert
	requireScreen(t, failed.View(), "┏━ git push", "✗ exit status 128")

	if calls := failing.asked("open "); len(calls) != 0 {
		t.Errorf("opened a pull request after the push failed: %q", calls)
	}
}

func TestARefusedPullRequestKeepsTheComposerOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := withoutPull()
	refusing.openErr = errAlreadyOpen
	model := refusing.live(t, 120, 40)

	// Act: open one the forge refuses
	refused := typing(t, model, "4", "n", keyEnter)

	// Assert: the composer stays open with the forge's reason
	requireScreen(t, refused.View(), "┏━ Open pull request", "✗ the forge rejected the request")

	// Act: close it
	closed := typing(t, refused, keyEsc).View()

	// Assert: the keyboard is back on the Review pane
	refuseScreen(t, closed, "┏━ Open pull request")
	requireScreen(t, closed, focused("4 Review"))
}

func TestADryRunOpensNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would open \""+pullTitle+"\" from "+featureName+" into main")

	if calls := append(dry.asked("open "), dry.asked("push")...); len(calls) != 0 {
		t.Errorf("a dry run pushed or opened: %q", calls)
	}
}

func TestCIIsAskedAgainWhileItRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	polling := newWorld()
	polling.ciInterval = time.Millisecond
	polling.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	// Act
	view := typing(t, polling.live(t, 120, 40), "4").View()

	// Assert
	requireScreen(t, view, "CI     ● passed")

	if checks := polling.asked("ci"); len(checks) != 3 {
		t.Errorf("checked CI %d times, want until it passed", len(checks))
	}
}

func TestFinishedCIIsAskedOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	settled := newWorld()
	settled.ciInterval = time.Millisecond

	// Act
	settled.live(t, 120, 40)

	// Assert
	if checks := settled.asked("ci"); len(checks) != 1 {
		t.Errorf("checked finished CI %d times, want once", len(checks))
	}
}

func TestRRefreshesTheReview(t *testing.T) {
	t.Parallel()

	// Arrange
	refreshing := newWorld()
	pane := typing(t, refreshing.live(t, 120, 40), "4")
	before := len(refreshing.asked("find"))

	// Act
	typing(t, pane, "r")

	// Assert
	if finds := len(refreshing.asked("find")); finds != before+1 {
		t.Errorf("looked for the pull request %d times, want once more than the %d before r", finds, before)
	}
}

func TestTheDryRunNoticeMentionsThePushForAnUnpushedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	dry.branch.Upstream = ""
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would push "+featureName+", then open")
}
