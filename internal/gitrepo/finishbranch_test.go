// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The three commands a finish runs, named so the test reads as the sequence.
const (
	finishSwitch = "git -C /work switch main"
	finishPull   = "git -C /work pull --ff-only"
	finishDelete = "git -C /work branch -D feat/token"
)

func TestFinishBranchSwitchesPullsAndDeletes(t *testing.T) {
	t.Parallel()

	// Arrange
	run, ran := recordingRunner(t, map[string]reply{
		finishSwitch: {},
		finishPull:   {},
		finishDelete: {},
	})
	repo := gitrepo.At(run, workDir)

	// Act
	err := repo.FinishBranch(t.Context(), "feat/token", "main")
	// Assert
	if err != nil {
		t.Fatalf("FinishBranch returned %v", err)
	}

	if want := []string{finishSwitch, finishPull, finishDelete}; !slices.Equal(*ran, want) {
		t.Errorf("ran %q, want the three commands in order", *ran)
	}
}

func TestFinishBranchStopsWhenAStepFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull cannot fast-forward, so the branch must not be deleted.
	run, ran := recordingRunner(t, map[string]reply{
		finishSwitch: {},
		finishPull:   {err: errNotIgnored},
	})
	repo := gitrepo.At(run, workDir)

	// Act
	err := repo.FinishBranch(t.Context(), "feat/token", "main")

	// Assert
	if err == nil {
		t.Fatal("FinishBranch returned nil, want the pull failure")
	}

	if slices.Contains(*ran, finishDelete) {
		t.Errorf("deleted the branch after the pull failed: %q", *ran)
	}
}
