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

			reviewing := newWorld()
			reviewing.ci = []forge.CI{tt.ci}

			view := typing(t, reviewing.live(t, 120, 40), "4").View()
			requireScreen(t, view, "#42 "+pullTitle, pullURL, "CI     "+tt.want)
		})
	}
}

func TestTheReviewPaneExplainsWhenThereIsNothingToShow(t *testing.T) {
	t.Parallel()

	onMain := newWorld()
	onMain.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	requireScreen(t, typing(t, onMain.live(t, 120, 40), "4").View(), "on no feature branch")

	if calls := onMain.asked("find"); len(calls) != 0 {
		t.Errorf("looked for a pull request from the base branch: %q", calls)
	}

	none := withoutPull()
	view := typing(t, none.live(t, 120, 40), "4").View()
	requireScreen(t, view, "no pull request yet", "n opens one from this branch's commits")
	requireScreen(t, footerLine(view), "n open pull request")

	unanswered := withoutPull()
	unanswered.pullErr = errUnreachable
	requireScreen(t, typing(t, unanswered.live(t, 120, 40), "4").View(),
		"✗ the forge did not answer", "✗ could not reach the forge")

	draft := newWorld()
	draft.pull.Draft = true
	requireScreen(t, typing(t, draft.live(t, 120, 40), "4").View(), "draft")
}

func TestNOpensAComposerStartedFromTheBranch(t *testing.T) {
	t.Parallel()

	opening := withoutPull()
	opening.templates = []forge.Template{
		{Name: "PULL_REQUEST_TEMPLATE", Body: "## What this changes\n"},
		{Name: "bugfix", Body: "## Bug\n"},
	}

	composer := typing(t, opening.live(t, 120, 40), "4", "n")
	requireScreen(t, composer.View(), "┏━ Open pull request", "title  > "+pullTitle, "base   > main",
		"head   "+featureName, "template PULL_REQUEST_TEMPLATE (1 of 2)", "[ ] draft", "## What this changes",
		"Jira: [PROJ-412](https://jira.example.com/browse/PROJ-412)")

	// Another template, a draft, a body from the editor.
	opening.edited = "Rewritten."
	changed := typing(t, composer, "ctrl+t", "ctrl+d")
	requireScreen(t, changed.View(), "template bugfix (2 of 2)", "## Bug", "[x] draft")

	edited := typing(t, changed, "ctrl+e")
	requireScreen(t, edited.View(), "Rewritten.")

	opened := typing(t, edited, keyEnter)
	requireScreen(t, opened.View(), "● opened #42 "+pullURL)

	want := "open " + pullTitle + " " + featureName + ">main draft=yes\nRewritten."
	if calls := opening.asked("open "); len(calls) != 1 || calls[0] != want {
		t.Errorf("open calls = %q, want %q", calls, want)
	}
}

func TestTheComposerTitleAndBaseCanBeEdited(t *testing.T) {
	t.Parallel()

	opening := withoutPull()

	keys := append([]string{"4", "n", "ctrl+e"}, letters("!")...)
	keys = append(append(keys, keyTab, "backspace", "backspace", "backspace", "backspace"), letters("develop")...)

	opening.edited = "Body."
	composed := typing(t, opening.live(t, 120, 40), keys...)
	requireScreen(t, composed.View(), "title  > "+pullTitle+"!", "base   > develop", "no template in this repository")

	typing(t, composed, keyEnter)

	wanted := "open " + pullTitle + "! " + featureName + ">develop"
	if calls := opening.asked("open "); len(calls) != 1 || !strings.HasPrefix(calls[0], wanted) {
		t.Errorf("open calls = %q", calls)
	}
}

func TestAPullRequestNeedsATitleAndABase(t *testing.T) {
	t.Parallel()

	opening := withoutPull()
	composer := typing(t, opening.live(t, 120, 40), "4", "n")

	erase := slices.Repeat([]string{"backspace"}, 60)

	untitled := typing(t, composer, append(slices.Clone(erase), keyEnter)...)
	requireScreen(t, untitled.View(), "✗ a pull request needs a title")

	baseless := typing(t, composer, append(append([]string{keyTab}, erase...), keyEnter)...)
	requireScreen(t, baseless.View(), "✗ a pull request needs a base branch to merge into")

	if calls := opening.asked("open "); len(calls) != 0 {
		t.Errorf("opened a pull request with a part missing: %q", calls)
	}
}

func TestAnUnpushedBranchIsPushedBeforeThePullRequestOpens(t *testing.T) {
	t.Parallel()

	unpushed := withoutPull()
	unpushed.branch.Upstream = ""

	opened := typing(t, unpushed.live(t, 120, 40), "4", "n", keyEnter)
	requireScreen(t, opened.View(), "● opened #42")

	pushes, opens := unpushed.asked("push"), unpushed.asked("open ")
	if len(pushes) != 1 || len(opens) != 1 {
		t.Errorf("push calls = %q, open calls = %d; want one of each", pushes, len(opens))
	}

	failing := withoutPull()
	failing.branch.Upstream = ""
	failing.pushErr = errPushDenied

	failed := typing(t, failing.live(t, 120, 40), "4", "n", keyEnter)
	requireScreen(t, failed.View(), "┏━ git push", "✗ exit status 128")

	if calls := failing.asked("open "); len(calls) != 0 {
		t.Errorf("opened a pull request after the push failed: %q", calls)
	}
}

func TestARefusedPullRequestKeepsTheComposerOpen(t *testing.T) {
	t.Parallel()

	refusing := withoutPull()
	refusing.openErr = errAlreadyOpen

	refused := typing(t, refusing.live(t, 120, 40), "4", "n", keyEnter)
	requireScreen(t, refused.View(), "┏━ Open pull request", "✗ the forge rejected the request")
	requireScreen(t, typing(t, refused, "esc").View(), focused("4 Review"))
}

func TestADryRunOpensNothing(t *testing.T) {
	t.Parallel()

	dry := withoutPull()

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "4", "n", keyEnter).View(),
		"dry run: would open \""+pullTitle+"\" from "+featureName+" into main")

	if calls := append(dry.asked("open "), dry.asked("push")...); len(calls) != 0 {
		t.Errorf("a dry run pushed or opened: %q", calls)
	}
}

func TestCIIsAskedAgainWhileItRuns(t *testing.T) {
	t.Parallel()

	polling := newWorld()
	polling.ciInterval = time.Millisecond
	polling.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}

	view := typing(t, polling.live(t, 120, 40), "4").View()
	requireScreen(t, view, "CI     ● passed")

	if checks := polling.asked("ci"); len(checks) != 3 {
		t.Errorf("checked CI %d times, want until it passed", len(checks))
	}

	settled := newWorld()
	settled.ciInterval = time.Millisecond
	settled.live(t, 120, 40)

	if checks := settled.asked("ci"); len(checks) != 1 {
		t.Errorf("checked finished CI %d times, want once", len(checks))
	}
}

func TestRRefreshesTheReview(t *testing.T) {
	t.Parallel()

	refreshing := newWorld()
	typing(t, refreshing.live(t, 120, 40), "4", "r")

	if finds := refreshing.asked("find"); len(finds) < 2 {
		t.Errorf("looked for the pull request %d times, want again on r", len(finds))
	}
}
