// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A refusal too long for its row is wrapped under the list, as the task
// line wraps Taskwarrior's, so the service's own words are all seen.

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// errLongRefusal is a refusal far wider than an overlay at 80 columns,
// whose last words must still be seen.
var errLongRefusal = errors.New("fatal: your local changes to the following files would be " +
	"overwritten by checkout: internal/tui/switchtask.go internal/tui/picker.go; " +
	"please commit your changes or stash them before you switch branches")

// railColumns is the rail's width at 80 columns, which the detail starts
// after.
const railColumns = 24

// requireWholly fails the test unless every word of err is in the detail,
// in order, however its rows are wrapped.
func requireWholly(t *testing.T, view string, err error) {
	t.Helper()

	var detail []string

	for line := range strings.SplitSeq(plain(view), "\n") {
		if runes := []rune(line); len(runes) > railColumns {
			detail = append(detail, string(runes[railColumns:]))
		}
	}

	if shown := strings.Join(strings.Fields(strings.Join(detail, " ")), " "); !strings.Contains(shown, err.Error()) {
		t.Errorf("the detail does not show %q in full:\n%s", err.Error(), plain(view))
	}
}

func TestTheSwitcherWrapsALongRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.checkoutErr = errLongRefusal
	screen := openSwitcher(t, repo.live(t, 80, 24))

	// Act
	view := typing(t, screen, keyEnter).View().Content

	// Assert
	requireWholly(t, view, errLongRefusal)
}

func TestTheStatusPickerWrapsALongRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.moves, repo.transitionErr = workflowMoves(), errLongRefusal

	// Act
	view := typing(t, repo.live(t, 80, 24), "t", keyEnter).View().Content

	// Assert
	requireWholly(t, view, errLongRefusal)
}

// refusedLastMove is the status picker at 80 by 24 over more transitions
// than it has rows for, the last chosen and refused in errLongRefusal's
// words.
func refusedLastMove(t *testing.T) tui.Model {
	t.Helper()

	repo := newWorld()
	repo.moves, repo.transitionErr = manyTransitions(), errLongRefusal
	keys := []string{"t"}

	for range len(repo.moves) - 1 {
		keys = append(keys, "j")
	}

	return typing(t, repo.live(t, 80, 24), append(keys, keyEnter)...)
}

func TestTheSwitcherWrapsALongRefusalUnderAFullList(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branches, repo.checkoutErr = manyTaskBranches(), errLongRefusal
	screen := openSwitcher(t, repo.live(t, 80, 24))

	// Act
	view := typing(t, screen, keyEnter).View().Content

	// Assert
	requireWholly(t, view, errLongRefusal)
}

func TestTheStatusPickerWrapsALongRefusalUnderAFullList(t *testing.T) {
	t.Parallel()

	// Act
	view := refusedLastMove(t).View().Content

	// Assert
	requireWholly(t, view, errLongRefusal)
}

func TestAClickAfterALongRefusalChoosesTheRowClicked(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := refusedLastMove(t)
	row := screenRow(t, refused.View().Content, "Step 20")

	// Act
	view := click(t, refused, railColumns+10, row).View().Content

	// Assert
	if line := lineShowing(t, view, "Step 20"); !strings.Contains(line, "▸ ") {
		t.Errorf("the row clicked, Step 20, is not chosen:\n%s", plain(view))
	}
}
