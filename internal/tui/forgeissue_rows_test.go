// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// forgeIssue is the forge issue listed ahead of Jira's, numbered as a forge
// numbers its issues.
const forgeIssue jira.Key = "57"

// withForgeIssue lists a forge issue ahead of the world's Jira issues; the
// branch's issue, below it, starts selected.
func withForgeIssue() *world {
	both := newWorld()
	both.issues = append([]jira.Issue{
		{Key: forgeIssue, Summary: "Typo in the README", Status: "Open", StatusCategory: categoryNew},
	}, both.issues...)

	return both
}

func TestAForgeIssueIsListedByItsNumber(t *testing.T) {
	t.Parallel()

	// Act
	view := withForgeIssue().live(t, placesViewWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, view, "#57 ")
}

func TestAForgeIssueOffersACommentButNoLogWork(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, wideFooterWidth, placesViewHeight), upAction).View().Content

	// Assert
	footer := footerLine(view)
	requireScreen(t, footer, "c comment", "a assign")
	refuseScreen(t, footer, "w log work")
}

func TestAssigningAForgeIssueStartsWithYourForgeName(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, placesViewWidth, placesViewHeight), upAction, "a").View().Content

	// Assert
	requireScreen(t, view, "Assign", "jacob")
}

func TestAssigningAForgeIssueStartsWithYourForgeNameOnABranchWithNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	unopened := withForgeIssue()
	unopened.pullFound = false

	// Act
	view := typing(t, unopened.live(t, placesViewWidth, placesViewHeight), upAction, "a").View().Content

	// Assert
	requireScreen(t, view, "Assign", "jacob")
}

func TestAForgeIssueKeepsItsHashWhereverItIsNamed(t *testing.T) {
	t.Parallel()

	assign := []string{upAction, "a", keyEnter}
	link := []string{"2", "i", "ctrl+u", "#", "4", "2", keyEnter}
	offConventionWorld := func() *world { return onOffConventionBranch(false) }

	cases := map[string]struct {
		repo func() *world
		dry  bool
		keys []string
		want string
	}{
		"the assign form's title": {repo: withForgeIssue, keys: []string{upAction, "a"}, want: "Assign #57"},
		"starting work":           {repo: withForgeIssue, keys: []string{upAction, "b"}, want: "Start work on #57"},
		"assigning":               {repo: withForgeIssue, keys: assign, want: "assigned #57 to jacob"},
		"assigning in a dry run": {
			repo: withForgeIssue, dry: true, keys: assign, want: "dry run: would assign #57 to jacob",
		},
		"linking a branch": {repo: offConventionWorld, keys: link, want: "linked my-thing to #42"},
		"linking a branch in a dry run": {
			repo: offConventionWorld, dry: true, keys: link, want: "dry run: would link my-thing to #42",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := tt.repo()

			model := tui.New(repo.cfg, nil, repo.deps())
			if tt.dry {
				model = model.WithDryRun()
			}

			started := sized(t, model, placesViewWidth, placesViewHeight)
			started = drain(t, started, started.Init())

			// Act
			view := typing(t, started, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestTheIssuesListSaysWhenTheForgeCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := newWorld()
	missing.unavailable = []string{"the forge's issues"}

	// Act
	view := missing.live(t, placesViewWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, view, "not read: the forge's issues")
}

// The "not read" line beneath a scrolled list takes a row from it, so a click
// has to count the rows the same way the drawing did.
func TestClickingAScrolledListAboveTheNotReadLineSelectsTheIssueClicked(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := newWorld()
	missing.issues = manyIssues(40)
	missing.unavailable = []string{"the forge's issues"}
	scrolled := typing(t, missing.live(t, 120, 40), slices.Repeat([]string{downAction}, 39)...)

	// Act
	picked := click(t, scrolled, 5, screenRow(t, scrolled.View().Content, "OPS-30 "))

	// Assert
	requireScreen(t, picked.View().Content, "▸ ○ OPS-30 ")
}

func TestTheWherePickerOffersTheForgesIssues(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "forge issue  1")
}
