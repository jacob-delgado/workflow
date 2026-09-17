// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// listBranches is the command LocalBranches runs: the local heads, most recently
// committed to first.
const listBranches = "git -C /work for-each-ref --format=%(refname:short) --sort=-committerdate refs/heads"

func TestLocalBranchesListsTheHeadsMostRecentFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		listBranches: {out: []byte("fix/PROJ-412-token\nmain\nchore/tidy\n")},
	}

	// Act
	branches, err := gitrepo.LocalBranches(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	want := []string{"fix/PROJ-412-token", "main", "chore/tidy"}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("LocalBranches = %q, %v, want %q", branches, err, want)
	}
}

// A branch name that would drive the terminal is left out: it is handed to git
// and drawn in a list, and a name like that is not one to switch to.
func TestLocalBranchesLeavesOutAnUnshowableName(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		listBranches: {out: []byte("main\nfix/\x1b]0;owned\x07evil\nchore/tidy\n")},
	}

	// Act
	branches, err := gitrepo.LocalBranches(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	want := []string{"main", "chore/tidy"}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("LocalBranches = %q, %v, want %q", branches, err, want)
	}
}

// errNoBranch is what git switch exits with when the branch does not exist.
var errNoBranch = errors.New("fatal: invalid reference")

func TestLocalBranchesReportsAFailureToList(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{listBranches: {err: errNoBranch}}

	// Act
	_, err := gitrepo.LocalBranches(t.Context(), fakeRunner(t, replies), workDir)

	// Assert
	if !errors.Is(err, errNoBranch) {
		t.Errorf("LocalBranches returned %v, want git's error", err)
	}
}

func TestCheckoutSwitchesToTheBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{"git -C /work switch fix/PROJ-412-token": {out: []byte("")}}

	// Act
	err := gitrepo.Checkout(t.Context(), fakeRunner(t, replies), workDir, "fix/PROJ-412-token")
	// Assert
	if err != nil {
		t.Errorf("Checkout returned %v, want nil", err)
	}
}

func TestCheckoutReportsAFailureToSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{"git -C /work switch gone": {err: errNoBranch}}

	// Act
	err := gitrepo.Checkout(t.Context(), fakeRunner(t, replies), workDir, "gone")

	// Assert
	if !errors.Is(err, errNoBranch) {
		t.Errorf("Checkout returned %v, want git's error", err)
	}
}
