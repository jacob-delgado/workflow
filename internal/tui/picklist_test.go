// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// longList is how many rows each long picker offers: more than an 80x24
// terminal's detail pane can show at once.
const longList = 30

// longPicker is a picker opened over longList rows: the world it needs, the
// keys that open it, and how its first and last rows read after the marker.
type longPicker struct {
	prepare     func(*world)
	open        []string
	first, last string
}

// longPickers is every list picker, each opened over a list too long to show
// whole.
func longPickers() map[string]longPicker {
	return map[string]longPicker{
		"status picker": {
			prepare: func(w *world) { w.moves = manyTransitions() },
			open:    []string{"t"},
			first:   "◐ Step 01", last: "◐ Step 30",
		},
		"task switcher": {
			prepare: func(w *world) { w.changes, w.branches = nil, manyTaskBranches() },
			open:    []string{"2", "s"},
			first:   "PROJ-101", last: "PROJ-130",
		},
		"fixup picker": {
			prepare: func(w *world) { w.branch.Commits, w.branch.Ahead = manyCommits(), longList },
			open:    []string{"3", "f"},
			first:   "0000030 feat: change 30", last: "0000001 feat: change 01",
		},
		"checks list": {
			prepare: func(w *world) { w.ci = []forge.CI{{State: forge.CIPassed, Checks: manyChecks()}} },
			open:    []string{"4", "c"},
			first:   "● check-01", last: "● check-30",
		},
		"a run's places": {
			prepare: func(w *world) { w.commitErr, w.commitLines = errHookFailed, manyPlaces() },
			open:    commitKeys("x"),
			first:   "f01.go:1 problem 01", last: "f30.go:30 problem 30",
		},
	}
}

func TestEveryPickerKeepsItsSelectionInSightPastTheEnd(t *testing.T) {
	t.Parallel()

	for name, picker := range longPickers() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			picker.prepare(faked)
			opened := typing(t, faked.live(t, 80, 24), picker.open...)

			// Act
			view := typing(t, opened, slices.Repeat([]string{"j"}, longList+5)...).View().Content

			// Assert
			requireScreen(t, view, "▸ "+picker.last)
			refuseScreen(t, view, picker.first)
		})
	}
}

func TestEveryPickerComesBackToItsFirstRowPastTheStart(t *testing.T) {
	t.Parallel()

	for name, picker := range longPickers() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			faked := newWorld()
			picker.prepare(faked)
			atTheEnd := typing(t, faked.live(t, 80, 24),
				slices.Concat(picker.open, slices.Repeat([]string{"j"}, longList+5))...)

			// Act
			view := typing(t, atTheEnd, slices.Repeat([]string{"k"}, longList+5)...).View().Content

			// Assert
			requireScreen(t, view, "▸ "+picker.first)
			refuseScreen(t, view, picker.last)
		})
	}
}

func TestAClickOnAScrolledListChoosesTheRowClicked(t *testing.T) {
	t.Parallel()

	// The two pickers a click can choose in, and a row near the end of each.
	cases := map[string]string{
		"status picker":  "◐ Step 29",
		"a run's places": "f29.go:29 problem 29",
	}

	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			picker := longPickers()[name]
			faked := newWorld()
			picker.prepare(faked)
			scrolled := typing(t, faked.live(t, 80, 24),
				slices.Concat(picker.open, slices.Repeat([]string{"j"}, longList+5))...)
			row := screenRow(t, scrolled.View().Content, target)

			// Act
			view := click(t, scrolled, 50, row).View().Content

			// Assert
			requireScreen(t, view, "▸ "+target)
		})
	}
}

func TestAClickAboveAScrolledListChoosesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	faked := newWorld()
	faked.moves = manyTransitions()
	scrolled := typing(t, faked.live(t, 80, 24),
		slices.Concat([]string{"t"}, slices.Repeat([]string{"j"}, longList+5))...)
	above := screenRow(t, scrolled.View().Content, "◐ Step") - 1

	// Act
	clicked := click(t, scrolled, 50, above)

	// Assert
	if clicked.View().Content != scrolled.View().Content {
		t.Errorf("a click above a scrolled list changed the screen:\n%s", clicked.View().Content)
	}
}

func TestAClickBelowALongListsWindowChoosesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	faked := newWorld()
	faked.moves = manyTransitions()
	opened := typing(t, faked.live(t, 80, 24), "t")
	below := lastScreenRow(t, opened.View().Content, "◐ Step") + 1

	// Act
	clicked := click(t, opened, 50, below)

	// Assert
	if clicked.View().Content != opened.View().Content {
		t.Errorf("a click below a long list's window changed the screen:\n%s", clicked.View().Content)
	}
}

// lastScreenRow is the last row a phrase renders on, so a click can land just
// past a list's window without counting how many rows it drew.
func lastScreenRow(t *testing.T, view, phrase string) int {
	t.Helper()

	for row, line := range slices.Backward(strings.Split(plain(view), "\n")) {
		if strings.Contains(line, phrase) {
			return row
		}
	}

	t.Fatalf("the screen does not show %q:\n%s", phrase, plain(view))

	return -1
}

// manyTransitions is longList transitions, Step 01 to Step 30.
func manyTransitions() []jira.Transition {
	moves := make([]jira.Transition, 0, longList)
	for index := 1; index <= longList; index++ {
		moves = append(moves, jira.Transition{
			ID: strconv.Itoa(index), Name: fmt.Sprintf("Step %02d", index), ToStatus: statusInProgress,
			ToStatusCategory: categoryIndeterminate,
		})
	}

	return moves
}

// manyTaskBranches is longList issue branches, PROJ-101 to PROJ-130.
func manyTaskBranches() []string {
	branches := make([]string, 0, longList)
	for index := 1; index <= longList; index++ {
		branches = append(branches, fmt.Sprintf("fix/PROJ-%d-task", 100+index))
	}

	return branches
}

// manyCommits is longList unpushed commits, oldest first as git lists them, so
// the picker shows change 30 first.
func manyCommits() []gitrepo.Commit {
	commits := make([]gitrepo.Commit, 0, longList)
	for index := 1; index <= longList; index++ {
		commits = append(commits, gitrepo.Commit{
			Hash: fmt.Sprintf("%07d", index), Subject: fmt.Sprintf("feat: change %02d", index),
		})
	}

	return commits
}

// manyChecks is longList passing checks, check-01 to check-30.
func manyChecks() []forge.Check {
	checks := make([]forge.Check, 0, longList)
	for index := 1; index <= longList; index++ {
		checks = append(checks, forge.Check{Name: fmt.Sprintf("check-%02d", index), State: forge.CIPassed})
	}

	return checks
}

// manyPlaces is a lint run's output pointing at longList places, f01.go:1 to
// f30.go:30.
func manyPlaces() []string {
	lines := []string{lintJobStarts}
	for index := 1; index <= longList; index++ {
		lines = append(lines, fmt.Sprintf("f%02d.go:%d:1: problem %02d", index, index, index))
	}

	return append(lines, "summary: (done in 1 seconds)", "🥊 lint (1 seconds)")
}
