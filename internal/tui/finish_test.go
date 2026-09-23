// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errFinishFailed is how a finish that cannot fast-forward reports the failure.
var errFinishFailed = errors.New("the base is behind")

// mergedBranch is newWorld with the branch's pull request merged, so the branch
// can be finished.
func mergedBranch() *world {
	w := newWorld()
	w.pull.State = forge.StateMerged

	return w
}

func TestTheReviewPaneShowsAMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged", "F finishes the branch")
}

func TestAMergedBranchDoesNotSitOnCheckingCI(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()

	// Act
	view := typing(t, merged.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "● merged")
	refuseScreen(t, view, "checking")
}

func TestFinishIsNotOfferedWithUnpushedCommits(t *testing.T) {
	t.Parallel()

	// Arrange
	// A commit was made on the merged branch after it was pushed, so a force
	// delete would lose it.
	merged := mergedBranch()
	merged.branch.Ahead = 1
	model := merged.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("offered to finish a branch with unpushed commits: %q", calls)
	}
}

func TestFinishIsNotOfferedWithoutABase(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch's base could not be found, so there is nowhere to switch to.
	merged := mergedBranch()
	merged.branch.Base = ""
	model := merged.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("offered to finish a branch with no base: %q", calls)
	}
}

func TestFinishRunsTheThreeCommandsOnAMergedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	done := typing(t, model, "4", "F", keyEnter)

	// Assert
	requireScreen(t, done.View().Content, "● finished "+featureName)

	if calls := merged.asked("finish " + featureName); len(calls) != 1 {
		t.Errorf("finish calls = %q, want one onto the base", merged.asked("finish"))
	}
}

func TestTheFinishPreviewShowsTheThreeCommands(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	preview := typing(t, model, "4", "F").View().Content

	// Assert
	requireScreen(t, preview, "git switch main", "git pull --ff-only", "git branch -D "+featureName)
}

func TestFinishIsNotOfferedOnAnOpenPull(t *testing.T) {
	t.Parallel()

	// Arrange
	// newWorld's pull request is open.
	open := newWorld()
	model := open.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "F")

	// Assert
	refuseScreen(t, after.View().Content, "git branch -D")

	if calls := open.asked("finish"); len(calls) != 0 {
		t.Errorf("finished an open pull request: %q", calls)
	}
}

func TestAFinishThatFailsSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	merged.finishErr = errFinishFailed
	model := merged.live(t, 120, 40)

	// Act
	failed := typing(t, model, "4", "F", keyEnter)

	// Assert
	requireScreen(t, failed.View().Content, "could not finish", "the base is behind")
}

func TestEscCancelsTheFinishPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := mergedBranch()
	model := merged.live(t, 120, 40)

	// Act
	canceled := typing(t, model, "4", "F", "x", keyEsc)

	// Assert
	refuseScreen(t, canceled.View().Content, "git branch -D")

	if calls := merged.asked("finish"); len(calls) != 0 {
		t.Errorf("finished after cancel: %q", calls)
	}
}

func TestTheFinishPreviewShowsItIsFinishing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The finish is never answered, so its in-flight preview can be seen, and a
	// key pressed then is ignored.
	previewing := typing(t, mergedBranch().live(t, 120, 40), "4", "F")

	// Act
	finishing, _ := pressed(t, previewing, keyEnter)
	finishing, _ = pressed(t, finishing, "x")

	// Assert
	requireScreen(t, finishing.View().Content, "finishing")
}

func TestADryRunFinishSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := mergedBranch()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	previewed := typing(t, model, "4", "F", keyEnter)

	// Assert
	requireScreen(t, previewed.View().Content, "dry run: would finish "+featureName)

	if calls := dry.asked("finish"); len(calls) != 0 {
		t.Errorf("a dry run finished: %q", calls)
	}
}
