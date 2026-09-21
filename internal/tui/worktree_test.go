// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
)

// keyCtrlW toggles the branch creator between a branch here and a worktree.
const keyCtrlW = "ctrl+w"

// errWorktreeExists is what git says when a worktree path is already taken.
var errWorktreeExists = errors.New("fatal: '/work-x' already exists")

func TestBranchingCanCreateAWorktreeAndSaysWhereItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "b", keyCtrlW, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "worktree for "+featureName, "/work-"+featureName)

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
	view := typing(t, model, "2", "b", keyCtrlW, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "New branch", "already exists")
}

func TestAWorktreeUnderDryRunCreatesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, dryInterface(repo), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "b", keyCtrlW, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run", "worktree")

	if made := repo.asked("worktree"); len(made) != 0 {
		t.Errorf("worktree calls = %v, want none under a dry run", made)
	}
}
