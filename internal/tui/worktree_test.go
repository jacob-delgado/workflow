// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// keyWorktree toggles the branch creator between a branch here and a worktree.
const keyWorktree = "ctrl+g"

// errWorktreeExists is what git says when a worktree path is already taken.
var errWorktreeExists = errors.New("fatal: '/work-x' already exists")

func TestCtrlWDeletesAWordWhileTheBranchIsNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append([]string{"2", "b"}, slices.Repeat([]string{keyBackspace}, 80)...)
	named := typing(t, newWorld().live(t, 120, 40), append(keys, letters("feat-x y")...)...)

	// Act
	view := typing(t, named, "ctrl+w").View().Content

	// Assert
	requireScreen(t, view, "> feat-x", "enter create")
	refuseScreen(t, view, "feat-x y", "create worktree")
}

func TestBranchingCanCreateAWorktreeAndSaysWhereItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "b", keyWorktree, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "created worktree for "+featureName, "/work-"+featureName)

	if made := repo.asked("worktree " + featureName); len(made) != 1 {
		t.Errorf("worktree calls = %v, want one for the branch", made)
	}

	if branched := repo.asked("create " + featureName); len(branched) != 0 {
		t.Errorf("also created a branch in place: %v", branched)
	}
}

func TestTheBranchCreatorOffersTheWorktreeToggle(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "b").View().Content

	// Assert
	requireScreen(t, footerLine(view), "as a worktree")
}

func TestAFailedWorktreeKeepsTheCreatorOpenWithTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.worktreeErr = errWorktreeExists
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "b", keyWorktree, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Start work on", "already exists")
}

func TestAWorktreeUnderDryRunCreatesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, dryInterface(repo), 160, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "b", keyWorktree, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would fetch origin, then create a worktree for "+featureName+" from origin/main")
	refuseScreen(t, view, "switch to it")

	if made := repo.asked("worktree"); len(made) != 0 {
		t.Errorf("worktree calls = %v, want none under a dry run", made)
	}
}

// madeWorktree is a world whose branch creator has just made a worktree
// for featureName, and the model as it was left.
func madeWorktree(t *testing.T) tui.Model {
	t.Helper()

	return typing(t, newWorld().live(t, 120, 40), "2", "b", keyWorktree, keyEnter)
}

func TestANewWorktreeOffersToSwitchToIt(t *testing.T) {
	t.Parallel()

	// Act
	view := madeWorktree(t).View().Content

	// Assert
	requireScreen(t, view, "Switch to the new worktree", "/work-"+featureName, "switch", "stay")
}

func TestEnterOnTheNewWorktreeOfferLeavesForIt(t *testing.T) {
	t.Parallel()

	// Act
	left, cmd := pressed(t, madeWorktree(t), keyEnter)

	// Assert
	if !quits(cmd) || left.Destination().Dir != "/work-"+featureName {
		t.Errorf("enter on the offer: quit %v, destination %q; want the program ended for the worktree",
			quits(cmd), left.Destination().Dir)
	}
}

func TestEscOnTheNewWorktreeOfferStays(t *testing.T) {
	t.Parallel()

	// Act
	stayed, cmd := pressed(t, madeWorktree(t), keyEsc)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("esc on the offer: quit %v, destination %q; want to stay", quits(cmd), stayed.Destination().Dir)
	}

	refuseScreen(t, stayed.View().Content, "Switch to the new worktree")
}
