// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// mergeable is newWorld with the pull request green, approved and clean, so it
// can be merged.
func mergeable() *world {
	w := newWorld()
	w.pull.Approvals = 1
	w.pull.Mergeable = forge.MergeClean

	return w
}

func TestTheReviewPaneShowsAPullReadyToMerge(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "1 approval", "mergeable")
}

func TestMergeMergesTheApprovedPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	model := reviewing.live(t, 120, 40)

	// Act
	merged := typing(t, model, "4", "M", keyEnter)

	// Assert
	requireScreen(t, merged.View().Content, "● merged #42")

	if calls := reviewing.asked("merge 42"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want one for the pull request", reviewing.asked("merge"))
	}
}

func TestTheMergePreviewOffersEachPermittedMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash, forge.MergeRebase}
	model := reviewing.live(t, 120, 40)

	// Act: open the preview
	preview := typing(t, model, "4", "M")

	// Assert: every permitted method is offered
	requireScreen(t, preview.View().Content, "Merge by:", "merge commit", "squash and merge", "rebase and merge")

	// Act: choose the second method and merge
	merged := typing(t, preview, "j", keyEnter)

	// Assert: it merged by the chosen method
	requireScreen(t, merged.View().Content, "● merged #42")

	if calls := reviewing.asked("merge 42 squash"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want a squash", reviewing.asked("merge 42"))
	}
}

func TestMergeIsNotOfferedWithoutApproval(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull is green and clean but has no approval.
	reviewing := newWorld()
	reviewing.pull.Mergeable = forge.MergeClean
	model := reviewing.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "M")

	// Assert
	refuseScreen(t, after.View().Content, "Merge by:")

	if calls := reviewing.asked("merge"); len(calls) != 0 {
		t.Errorf("merged an unapproved pull request: %q", calls)
	}
}

func TestARefusedMergeNamesTheMissingScope(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeErr = forge.ErrRefused
	model := reviewing.live(t, 120, 40)

	// Act
	refused := typing(t, model, "4", "M", keyEnter)

	// Assert
	requireScreen(t, refused.View().Content, "could not merge", "the token needs a write scope")
	refuseScreen(t, refused.View().Content, "Merge by:")
}

func TestMergeSaysWhyTheMethodsCannotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeMethodsErr = forge.ErrRefused
	model := reviewing.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "M")

	// Assert
	requireScreen(t, after.View().Content, "cannot merge")
	refuseScreen(t, after.View().Content, "Merge by:")
}

func TestMergeSaysWhenTheRepositoryPermitsNoMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{}
	model := reviewing.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "M")

	// Assert
	requireScreen(t, after.View().Content, "permits no merge method")
}

func TestAMergeThatFailsSurfacesTheForgesReason(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge rejects the merge with a reason of its own, not a missing scope.
	reviewing := mergeable()
	reviewing.mergeErr = fmt.Errorf("%w: the base branch moved on", forge.ErrRejected)
	model := reviewing.live(t, 120, 40)

	// Act
	failed := typing(t, model, "4", "M", keyEnter)

	// Assert
	requireScreen(t, failed.View().Content, "the base branch moved on")
	refuseScreen(t, failed.View().Content, "the token needs a write scope", "the forge did not answer", "Merge by:")
}

func TestMergeIsNotOfferedOnADraft(t *testing.T) {
	t.Parallel()

	// Arrange
	// The draft is otherwise mergeable, but a draft cannot be merged.
	reviewing := mergeable()
	reviewing.pull.Draft = true
	model := reviewing.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "M")

	// Assert
	refuseScreen(t, after.View().Content, "Merge by:")

	if calls := reviewing.asked("merge"); len(calls) != 0 {
		t.Errorf("offered merge on a draft: %q", calls)
	}
}

func TestEscCancelsTheMergePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	model := reviewing.live(t, 120, 40)

	// Act
	canceled := typing(t, model, "4", "M", keyEsc)

	// Assert
	refuseScreen(t, canceled.View().Content, "Merge by:")

	if calls := reviewing.asked("merge 42"); len(calls) != 0 {
		t.Errorf("merged after cancel: %q", calls)
	}
}

func TestTheMergePreviewMovesBetweenMethods(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash}
	model := reviewing.live(t, 120, 40)

	// Act
	merged := typing(t, model, "4", "M", "j", "x", "k", keyEnter)

	// Assert
	requireScreen(t, merged.View().Content, "● merged #42")

	if calls := reviewing.asked("merge 42 merge"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want the first method after moving back", reviewing.asked("merge 42"))
	}
}

func TestTheMergePreviewShowsItIsMerging(t *testing.T) {
	t.Parallel()

	// Arrange
	// The merge is held open so the in-flight preview can be seen, and a key
	// pressed then is ignored rather than starting a second merge.
	reviewing := mergeable()
	reviewing.mergeGate = make(chan struct{})
	model := reviewing.live(t, 120, 40)

	// Act
	merging := typing(t, model, "4", "M", keyEnter, "j")

	// Assert
	requireScreen(t, merging.View().Content, "merging")

	close(reviewing.mergeGate)
}

func TestADryRunMergeSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := mergeable()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	previewed := typing(t, model, "4", "M", keyEnter)

	// Assert
	requireScreen(t, previewed.View().Content, "dry run: would merge #42 by merge commit")

	if calls := dry.asked("merge 42"); len(calls) != 0 {
		t.Errorf("a dry run merged: %q", calls)
	}
}
