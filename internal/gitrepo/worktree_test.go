// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errWorktreeExists is what git says when a worktree path is already taken.
var errWorktreeExists = errors.New("fatal: '/work-dup' already exists")

func TestWorktreeAddCreatesABranchBesideTheRepo(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		"git -C /work worktree add --no-track -b fix/PROJ-1 /work-fix-PROJ-1 origin/main": {out: []byte("")},
	}

	// Act
	path, err := gitrepo.At(fakeRunner(t, replies), workDir).WorktreeAdd(t.Context(), "fix/PROJ-1", "origin/main")

	// Assert
	if err != nil || path != "/work-fix-PROJ-1" {
		t.Errorf("WorktreeAdd = %q, %v, want the sibling path", path, err)
	}
}

func TestWorktreeAddFromHeadOmitsTheStart(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{"git -C /work worktree add -b feat/x /work-feat-x": {out: []byte("")}}

	// Act
	path, err := gitrepo.At(fakeRunner(t, replies), workDir).WorktreeAdd(t.Context(), "feat/x", "")

	// Assert
	if err != nil || path != "/work-feat-x" {
		t.Errorf("WorktreeAdd = %q, %v, want the branch from HEAD", path, err)
	}
}

func TestWorktreeAddReportsAFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		"git -C /work worktree add --no-track -b dup /work-dup origin/main": {err: errWorktreeExists},
	}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).WorktreeAdd(t.Context(), "dup", "origin/main")

	// Assert
	if !errors.Is(err, errWorktreeExists) {
		t.Errorf("WorktreeAdd returned %v, want git's error", err)
	}
}
