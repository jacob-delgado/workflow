// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// changeStatusHint is the Issues footer's offer to change the selected issue's
// status.
const changeStatusHint = "t change status"

// wideFooter is the Issues pane's footer on a terminal wide enough to hold every
// key it offers, so a key missing from it was not offered rather than cut.
func wideFooter(t *testing.T, deps tui.Deps, keys ...string) string {
	t.Helper()

	model := sized(t, tui.New(completeConfig(), nil, deps), 200, 40)

	return footerLine(typing(t, drain(t, model, model.Init()), keys...).View().Content)
}

func TestIssuesFooterShowsEveryLiveKey(t *testing.T) {
	t.Parallel()

	// Act
	footer := wideFooter(t, newWorld().deps())

	// Assert
	requireScreen(t, footer, changeStatusHint, "c comment", "a assign", "w log work",
		"b branch for PROJ-412", "/ filter", "o open", "y copy url", "r refresh", "? keys")
}

func TestTheIssuesFooterOffersAVerbOnlyWhereItsSeamIsWired(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*tui.Deps)
		absent string
	}{
		"no transitions":       {unwire: func(deps *tui.Deps) { deps.Jira.Transitions = nil }, absent: changeStatusHint},
		"no comment write":     {unwire: func(deps *tui.Deps) { deps.Jira.Comment = nil }, absent: "c comment"},
		"no editor":            {unwire: func(deps *tui.Deps) { deps.Editor.Edit = nil }, absent: "c comment"},
		"no assign write":      {unwire: func(deps *tui.Deps) { deps.Jira.Assign = nil }, absent: "a assign"},
		"no worklog write":     {unwire: func(deps *tui.Deps) { deps.Jira.AddWorklog = nil }, absent: "w log work"},
		"no branch creator":    {unwire: func(deps *tui.Deps) { deps.Git.CreateBranch = nil }, absent: "b branch for"},
		"outside a repository": {unwire: outsideARepository, absent: "b branch for"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := newWorld().deps()
			tt.unwire(&deps)

			// Act
			footer := wideFooter(t, deps)

			// Assert
			refuseScreen(t, footer, tt.absent)
			requireScreen(t, footer, "r refresh")
		})
	}
}

func TestTheIssuesFooterOffersTheFilterOnlyOverAList(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.issues = nil

	// Act
	footer := wideFooter(t, empty.deps())

	// Assert
	refuseScreen(t, footer, "/ filter")
}

func TestSlashOpensNoFilterOverAnEmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.issues = nil
	listed := empty.live(t, 120, 30)

	// Act
	after := typing(t, listed, "/")

	// Assert
	refuseScreen(t, after.View().Content, "filter:")
}

func TestAFilterThatMatchesNothingStillOffersANewOne(t *testing.T) {
	t.Parallel()

	// Act
	footer := wideFooter(t, newWorld().deps(), "/", "z", "z", keyEnter)

	// Assert
	requireScreen(t, footer, "/ filter")
	refuseScreen(t, footer, changeStatusHint)
}

func TestWithNoIssueSelectedTheIssuesFooterOffersABranchWhereOneCanStart(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		unwire func(*tui.Deps)
		shows  bool
	}{
		"a branch creator": {unwire: func(*tui.Deps) {}, shows: true},
		"no branch creator": {
			unwire: func(deps *tui.Deps) { deps.Git.CreateBranch = nil }, shows: false,
		},
		"outside a repository": {unwire: outsideARepository, shows: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			empty := newWorld()
			empty.issues = nil
			deps := empty.deps()
			tt.unwire(&deps)

			// Act
			footer := wideFooter(t, deps)

			// Assert
			if shown := strings.Contains(footer, "b new branch"); shown != tt.shows {
				t.Errorf("the footer offers b new branch: %t, want %t:\n%s", shown, tt.shows, footer)
			}
		})
	}
}

func TestWithNoIssueSelectedTheBranchKeyStartsABranchForNoIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.issues = nil

	// Act
	view := typing(t, empty.live(t, 120, 40), "b").View().Content

	// Assert
	// The footer offered b new branch, and b opens the creator, for no issue.
	requireScreen(t, view, "New branch")
	refuseScreen(t, view, "for PROJ")
}

func TestOutsideARepositoryTheBranchKeyOpensNothing(t *testing.T) {
	t.Parallel()

	for name, pane := range map[string]string{"the Issues pane": "1", "the Branch pane": "2"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := newWorld().deps()
			outsideARepository(&deps)
			model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, pane, "b").View().Content

			// Assert
			refuseScreen(t, view, "New branch")
		})
	}
}

func TestWhileTheFilterIsTypedTheFooterOffersOnlyKeepingOrClearingIt(t *testing.T) {
	t.Parallel()

	// Act
	footer := wideFooter(t, newWorld().deps(), "/")

	// Assert
	// Every other key types into the filter, so the footer offers none of them.
	requireScreen(t, footer, "enter keep filter", "esc clear filter")
	refuseScreen(t, footer, changeStatusHint, "? keys", "q quit")
}

func TestTheFilterFooterNamesTheKeysTheFilterReadsWhateverApplyAndCloseAreBoundTo(t *testing.T) {
	t.Parallel()

	// Arrange
	// Enter and esc keep and clear the filter as it is typed, whatever apply and
	// close are moved to, so a moved printable key can still be typed into it.
	rebound := newWorld()
	rebound.cfg.UI.Keys = map[string]string{"apply": "ctrl+s", "close": "ctrl+g"}

	// Act
	footer := footerLine(typing(t, rebound.live(t, 200, 40), "/").View().Content)

	// Assert
	requireScreen(t, footer, "enter keep filter", "esc clear filter")
	refuseScreen(t, footer, "ctrl+s", "ctrl+g")
}

func TestTheCollapsedIssuesFooterOffersReadingAnIssueAndGoingBack(t *testing.T) {
	t.Parallel()

	// Arrange
	scanning := newWorld().live(t, 60, 30)

	// Act
	reading := typing(t, scanning, keyEnter)

	// Assert
	// Scanning offered reading the issue, and reading it offers the way back.
	requireScreen(t, footerLine(scanning.View().Content), "enter read issue")
	requireScreen(t, footerLine(reading.View().Content), "esc back to list")
	refuseScreen(t, footerLine(reading.View().Content), "enter read issue")
}

func TestEscInTheCollapsedIssuesGoesBackToTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	reading := typing(t, newWorld().live(t, 60, 30), keyEnter)

	// Act
	back := typing(t, reading, keyEsc).View().Content

	// Assert
	// The list is back: the footer offers reading the issue again, and the
	// issue's description is gone.
	requireScreen(t, footerLine(back), "enter read issue")
	refuseScreen(t, back, "Tokens reach the log.")
}

func TestBesideTheRailTheIssuesFooterOffersNoReadingKeys(t *testing.T) {
	t.Parallel()

	// Act
	// The detail already shows the selected issue beside the list, so enter and
	// esc have nothing to switch between.
	footer := wideFooter(t, newWorld().deps(), keyEnter)

	// Assert
	refuseScreen(t, footer, "read issue", "back to list")
}
