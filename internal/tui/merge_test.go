// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// readingMethods is the merge picker while the methods it offers are read.
const readingMethods = "loading merge methods…"

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
	// pinned in it with no rate-limit guess, ready to try again
	requireScreen(t, refused.View().Content, "Merge by:",
		"✗ The forge refused the write: the token may lack the write scope", "enter merge")
	refuseScreen(t, refused.View().Content, "rate limit")

	// Act: close it
	closed := typing(t, refused, keyEsc)

	// Assert: the preview is gone, and the refused merge was asked for once
	refuseScreen(t, closed.View().Content, "Merge by:", lacksWriteScope)

	if calls := reviewing.asked("merge 42"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want the one refused", calls)
	}
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestAMethodsReadThatFailsIsPinnedInTheMergePicker(t *testing.T) {
	defer forceANSI(t)()

	cases := map[string]struct {
		prepare func(*world)
		want    string
	}{
		"methods the token may not read": {
			prepare: func(w *world) { w.mergeMethodsErr = forge.ErrRefused },
			want:    "The forge refused the request",
		},
		"methods the forge would not show": {
			prepare: func(w *world) { w.mergeMethodsErr = forge.ErrUnreachable },
			want:    "The forge did not answer in time.",
		},
		"a repository that permits no merge method": {
			prepare: func(w *world) { w.mergeMethods = []forge.MergeMethod{} },
			want:    "cannot merge: the repository permits no merge method",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			reviewing := mergeable()
			tt.prepare(reviewing)

			// Act
			view := typing(t, reviewing.live(t, 200, 40), "4", "M").View().Content

			// Assert
			requireFailureRow(t, view, tt.want)
			requireScreen(t, view, "Merge pull request", "esc cancel")
			refuseScreen(t, view, "Merge by:", "enter merge")
		})
	}
}

func TestTheMergePickerNamesTheMergeOnANarrowTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeable()
	reviewing.mergeMethodsErr = fmt.Errorf("listing merge methods: %w", forge.ErrRefused)

	// Act
	view := typing(t, reviewing.live(t, 80, 30), "4", "M").View().Content

	// Assert
	// The row clips a long sentence, so the picker's title is what names the
	// merge the failure stopped.
	requireScreen(t, view, "Merge pull request", "✗ The forge refused the request")
}

func TestTheMergePickerSaysItIsReadingTheMethods(t *testing.T) {
	t.Parallel()

	// Arrange
	onReview := typing(t, mergeable().live(t, 120, 40), "4")

	// Act
	// The read is never run, so the methods are still out.
	reading, _ := pressed(t, onReview, "M")

	// Assert
	requireScreen(t, reading.View().Content, "Merge pull request", readingMethods, "esc cancel")
	refuseScreen(t, reading.View().Content, "Merge by:", "enter merge")
}

func TestAMergePickerWithNoMethodTakesOnlyEsc(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answered bool
		key      string
		showing  string
	}{
		"enter while the methods are read":       {key: keyEnter, showing: readingMethods},
		"j while the methods are read":           {key: "j", showing: readingMethods},
		"e while the methods are read":           {key: "e", showing: readingMethods},
		"enter once the repository permits none": {answered: true, key: keyEnter, showing: "permits no merge method"},
		"j once the repository permits none":     {answered: true, key: "j", showing: "permits no merge method"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			reviewing := mergeable()
			reviewing.mergeMethods = []forge.MergeMethod{}
			picker, read := pressed(t, typing(t, reviewing.live(t, 120, 40), "4"), "M")

			if tt.answered {
				picker, _ = finish(t, picker, read)
			}

			// Act
			after, cmd := pressed(t, picker, tt.key)

			// Assert
			requireScreen(t, after.View().Content, "Merge pull request", tt.showing)

			if cmd != nil {
				t.Errorf("%q started work in a merge picker with no method to choose", tt.key)
			}
		})
	}
}

func TestADownKeyWhileTheMethodsAreReadLeavesTheFirstChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	// j is pressed while the methods are still out, and they land after it.
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash}
	picker, read := pressed(t, typing(t, reviewing.live(t, 120, 40), "4"), "M")
	moved, _ := pressed(t, picker, "j")
	loaded, _ := finish(t, moved, read)

	// Act
	typing(t, loaded, keyEnter)

	// Assert
	if calls := reviewing.asked("merge 42 merge"); len(calls) != 1 {
		t.Errorf("merge calls = %q, want the first method", reviewing.asked("merge 42"))
	}
}

func TestAMethodsAnswerLeavesAnOverlayOpenedMeanwhile(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"the pull request editor": {keys: []string{keyEsc, "e"}, want: "Edit pull request"},
		"nothing at all":          {keys: []string{keyEsc}, want: "mergeable"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The picker is left while its methods are still out, and something
			// else is opened in its place before they land.
			asking, read := pressed(t, typing(t, mergeable().live(t, 120, 40), "4"), "M")
			meanwhile := typing(t, asking, tt.keys...)

			// Act
			answered, _ := finish(t, meanwhile, read)

			// Assert
			requireScreen(t, answered.View().Content, tt.want)
			refuseScreen(t, answered.View().Content, "Merge by:", readingMethods)
		})
	}
}

func TestALateMethodsAnswerKeepsTheChoiceBeingMade(t *testing.T) {
	t.Parallel()

	// Arrange
	// The picker is closed and reopened, so two reads are out; the first to land
	// fills it, and a method is chosen before the second lands with other methods.
	reviewing := mergeable()
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeCommit, forge.MergeSquash}
	first, firstRead := pressed(t, typing(t, reviewing.live(t, 120, 40), "4"), "M")
	reopened, secondRead := pressed(t, typing(t, first, keyEsc), "M")
	filled, _ := finish(t, reopened, firstRead)
	choosing := typing(t, filled, "j")
	reviewing.mergeMethods = []forge.MergeMethod{forge.MergeRebase}

	// Act
	answered, _ := finish(t, choosing, secondRead)

	// Assert
	requireScreen(t, answered.View().Content, "▸ squash and merge")
	refuseScreen(t, answered.View().Content, "rebase and merge")
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
