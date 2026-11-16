// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// goneRefusal is what the switcher says of a branch held by a worktree whose
// directory is gone.
const goneRefusal = "git worktree prune"

// goneAndFree is the switcher's world with two task branches: the first held
// by a worktree whose directory is gone, the second free to switch to.
func goneAndFree() *world {
	working := retriesInAWorktree()
	working.dirs.worktrees[1].Missing = true
	working.branches = append(working.branches, "fix/PROJ-501-fix-metrics")

	return working
}

func TestAGoneWorktreesRefusalGivesWayToTheSwitchSentAfterIt(t *testing.T) {
	t.Parallel()

	// Arrange
	working := goneAndFree()
	working.checkoutErr = errWouldOverwrite
	refused := typing(t, working.live(t, 160, 40), "2", "s", keyEnter)

	// Act
	view := typing(t, refused, "j", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "would be overwritten")
	refuseScreen(t, view, goneRefusal)
}

func TestAGoneWorktreesRefusalTakesThePlaceOfAnEarlierOne(t *testing.T) {
	t.Parallel()

	// Arrange
	working := goneAndFree()
	working.changes = editedNotes()
	refused := typing(t, working.live(t, 160, 40), "2", "s", "j", keyEnter)

	// Act
	view := typing(t, refused, "k", keyEnter).View().Content

	// Assert
	requireScreen(t, view, goneRefusal)
	refuseScreen(t, view, loop.ErrDirtyTree.Error())
}

func TestMovingOffABranchHeldByAGoneWorktreeClearsItsRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := typing(t, goneAndFree().live(t, 160, 40), "2", "s", keyEnter)

	// Act
	view := typing(t, refused, "j").View().Content

	// Assert
	requireScreen(t, view, "fix/PROJ-501-fix-metrics")
	refuseScreen(t, view, goneRefusal)
}

func TestMovingPastTheTopOfTheListKeepsAGoneWorktreesRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	refused := typing(t, goneAndFree().live(t, 160, 40), "2", "s", keyEnter)

	// Act
	view := typing(t, refused, "k").View().Content

	// Assert
	requireScreen(t, view, goneRefusal)
}
