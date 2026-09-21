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
	branches, err := gitrepo.At(fakeRunner(t, replies), workDir).LocalBranches(t.Context())

	// Assert
	want := []string{"fix/PROJ-412-token", localMain, "chore/tidy"}
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
	branches, err := gitrepo.At(fakeRunner(t, replies), workDir).LocalBranches(t.Context())

	// Assert
	want := []string{localMain, "chore/tidy"}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("LocalBranches = %q, %v, want %q", branches, err, want)
	}
}

// listRemotes is the command RemoteBranches runs: the remote-tracking refs, most
// recently committed to first.
const listRemotes = "git -C /work for-each-ref --format=%(refname:short) --sort=-committerdate refs/remotes"

func TestRemoteBranchesListThemByNameMostRecentFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		listRemotes: {out: []byte("origin/main\norigin/release/1.2\norigin/develop\n")},
	}

	// Act
	branches, err := gitrepo.At(fakeRunner(t, replies), workDir).RemoteBranches(t.Context())

	// Assert
	// The remote prefix is dropped, so the names are the ones a base carries, and
	// a branch name with a slash of its own keeps it.
	want := []string{localMain, "release/1.2", "develop"}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("RemoteBranches = %q, %v, want %q", branches, err, want)
	}
}

func TestRemoteBranchesDropTheHeadPointerAndDeduplicate(t *testing.T) {
	t.Parallel()

	// Arrange
	// git abbreviates a remote's symbolic HEAD pointer to the bare remote name
	// ("origin"), so that is the form the code meets; "origin/HEAD" is included
	// too for the defensive guard. main is on two remotes and must show once.
	replies := map[string]reply{
		listRemotes: {out: []byte("origin\norigin/HEAD\norigin/main\nupstream/main\nupstream/spike\n")},
	}

	// Act
	branches, err := gitrepo.At(fakeRunner(t, replies), workDir).RemoteBranches(t.Context())

	// Assert
	want := []string{localMain, "spike"}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("RemoteBranches = %q, %v, want %q", branches, err, want)
	}
}

func TestRemoteBranchesLeaveOutAnUnshowableName(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{
		listRemotes: {out: []byte("origin/main\norigin/fix/\x1b]0;owned\x07evil\n")},
	}

	// Act
	branches, err := gitrepo.At(fakeRunner(t, replies), workDir).RemoteBranches(t.Context())

	// Assert
	want := []string{localMain}
	if err != nil || !slices.Equal(branches, want) {
		t.Errorf("RemoteBranches = %q, %v, want %q", branches, err, want)
	}
}

func TestRemoteBranchesReportAFailureToList(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{listRemotes: {err: errNoBranch}}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).RemoteBranches(t.Context())

	// Assert
	if !errors.Is(err, errNoBranch) {
		t.Errorf("RemoteBranches returned %v, want git's error", err)
	}
}

// errNoBranch is what git switch exits with when the branch does not exist.
var errNoBranch = errors.New("fatal: invalid reference")

func TestLocalBranchesReportsAFailureToList(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := map[string]reply{listBranches: {err: errNoBranch}}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).LocalBranches(t.Context())

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
	err := gitrepo.At(fakeRunner(t, replies), workDir).Checkout(t.Context(), "fix/PROJ-412-token")
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
	err := gitrepo.At(fakeRunner(t, replies), workDir).Checkout(t.Context(), "gone")

	// Assert
	if !errors.Is(err, errNoBranch) {
		t.Errorf("Checkout returned %v, want git's error", err)
	}
}
