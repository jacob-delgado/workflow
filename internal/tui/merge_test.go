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

func TestRefusedMergeStaysInItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeErr = forge.ErrRefused
	preview := typing(t, reviewing.live(t, 120, 40), "4", "M")

	// Act: merge, and the forge refuses
	refused := typing(t, preview, keyEnter)

	// Assert: the preview is still open, the write scope the token may lack
	// pinned in it beside the forge's own hedge, ready to try again
	requireScreen(t, refused.View().Content, "Merge by:",
		"✗ The forge refused the write: the token may lack the write scope", "may be rate limiting", "enter merge")

	// Act: close it
	closed := typing(t, refused, keyEsc)

	// Assert: the preview is gone, and the refused merge was asked for once
	refuseScreen(t, closed.View().Content, "Merge by:", lacksWriteScope)

	if calls := reviewing.asked("merge 42"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want the one refused", calls)
	}
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

func TestARefusedMergeCanBeTriedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeErr = forge.ErrRefused
	refused := typing(t, reviewing.live(t, 120, 40), "4", "M", keyEnter)
	reviewing.mergeErr = nil

	// Act
	retried := typing(t, refused, keyEnter)

	// Assert
	requireScreen(t, retried.View().Content, "● merged #42")
	refuseScreen(t, retried.View().Content, "Merge by:")

	if calls := reviewing.asked("merge 42"); len(calls) != 2 {
		t.Errorf("merge calls = %q, want the refused merge and its retry", calls)
	}
}

func TestAMergeThatFailsSurfacesTheForgesReason(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err    error
		want   string
		refuse []string
	}{
		"the forge's own reason": {
			err:    fmt.Errorf("%w: the base branch moved on", forge.ErrRejected),
			want:   "✗ the forge rejected the request: the base branch moved on",
			refuse: []string{lacksWriteScope},
		},
		"a credential the forge did not accept": {
			err:  forge.ErrUnauthorized,
			want: "✗ The forge refused the write: the token may lack the write scope",
		},
		// The forge's own words, not a paraphrase that drops the cause.
		"a forge that never answered": {
			err:    fmt.Errorf("%w: dial tcp: i/o timeout", forge.ErrUnreachable),
			want:   "dial tcp: i/o timeout",
			refuse: []string{lacksWriteScope},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reviewing := mergeable()
			reviewing.mergeErr = tt.err

			// Act
			failed := typing(t, reviewing.live(t, 120, 40), "4", "M", keyEnter)

			// Assert
			requireScreen(t, failed.View().Content, "Merge by:", tt.want)
			refuseScreen(t, failed.View().Content, tt.refuse...)
		})
	}
}

func TestMergeIsNotOfferedOnceThePullMerges(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull is green and approved — merge would be offered — until it merges
	// under the reader, leaving the pane's last-read CI still green.
	reviewing := mergeable()
	onReview := typing(t, reviewing.live(t, 120, 40), "4")
	reviewing.pull.State = forge.StateMerged

	// Act
	refreshed := typing(t, onReview, "r", "M")

	// Assert
	refuseScreen(t, refreshed.View().Content, "Merge by:")

	if calls := reviewing.asked("merge"); len(calls) != 0 {
		t.Errorf("offered to merge an already-merged pull request: %q", calls)
	}
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
	// The merge is never answered, so the in-flight preview can be seen, and a
	// key pressed then is ignored rather than starting a second merge.
	previewing := typing(t, mergeable().live(t, 120, 40), "4", "M")

	// Act
	merging, _ := pressed(t, previewing, keyEnter)
	merging, _ = pressed(t, merging, "j")

	// Assert
	requireScreen(t, merging.View().Content, "merging")
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
