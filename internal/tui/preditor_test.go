// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// errEditRefused is how the forge turns an edit down.
var errEditRefused = errors.New("the token cannot edit this pull request")

func TestTheReviewPaneOffersEditingThePullRequest(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "e edits its title and description")
}

func TestEditIsNotOfferedWithoutAPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// withoutPull finds no pull request, so there is nothing to edit.
	model := withoutPull().live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "e")

	// Assert
	refuseScreen(t, footerLine(after.View().Content), "e edit")
	refuseScreen(t, after.View().Content, "Edit pull request")
}

func TestEditingKeepsTheExistingDescription(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request has a description a title-only edit must not wipe.
	editing := newWorld()
	editing.pull.Body = "keep this description"
	model := editing.live(t, 120, 40)

	// Act
	typing(t, model, "4", "e", keyEnter)

	// Assert
	if calls := editing.asked("edit 42 " + pullTitle + "\nkeep this description"); len(calls) != 1 {
		t.Errorf("edit calls = %q, want the existing description kept", editing.asked("edit"))
	}
}

func TestEditingUpdatesTheTitleEverywhere(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	model := editing.live(t, 120, 40)

	// Act
	saved := typing(t, model, append([]string{"4", "e"}, append(letters(" refined"), keyEnter)...)...)

	// Assert
	// The pane reflects the new title without a refresh, and the forge got it.
	requireScreen(t, saved.View().Content, pullTitle+" refined")

	if calls := editing.asked("edit 42 " + pullTitle + " refined"); len(calls) != 1 {
		t.Errorf("edit calls = %q, want the new title sent", editing.asked("edit"))
	}
}

func TestEditingTheBodyThenSavingSendsTheNewBody(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	editing.edited = "A fuller description."
	model := editing.live(t, 120, 40)

	// Act
	typing(t, model, "4", "e", "ctrl+o", keyEnter)

	// Assert
	if calls := editing.asked("edit 42 " + pullTitle + "\nA fuller description."); len(calls) != 1 {
		t.Errorf("edit calls = %q, want the new body sent", editing.asked("edit"))
	}
}

func TestSavingRefusesAnEmptyTitle(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	model := editing.live(t, 120, 40)

	// Act
	// Clear the title, then try to save.
	keys := append([]string{"4", "e"}, slices.Repeat([]string{keyBackspace}, 80)...)
	after := typing(t, model, append(keys, keyEnter)...)

	// Assert
	requireScreen(t, after.View().Content, "a title is required", "Edit pull request")

	if calls := editing.asked("edit"); len(calls) != 0 {
		t.Errorf("edited with an empty title: %q", calls)
	}
}

func TestAnEditThatFailsKeepsTheEditorOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	editing.editPullErr = errEditRefused
	model := editing.live(t, 120, 40)

	// Act
	failed := typing(t, model, "4", "e", keyEnter)

	// Assert
	// The reason shows, the editor stays open to retry, and nothing is applied.
	requireScreen(t, failed.View().Content, "cannot edit this pull request", "Edit pull request")
	refuseScreen(t, failed.View().Content, "● updated")
}

func TestEscLeavesTheEditorWithoutSaving(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	model := editing.live(t, 120, 40)

	// Act
	left := typing(t, model, "4", "e", keyEsc)

	// Assert
	refuseScreen(t, left.View().Content, "Edit pull request")

	if calls := editing.asked("edit"); len(calls) != 0 {
		t.Errorf("saved after esc: %q", calls)
	}
}

func TestAFailedEditorKeepsTheEditorOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	editing.editErr = errEditorFailed
	model := editing.live(t, 120, 40)

	// Act
	after := typing(t, model, "4", "e", "ctrl+o")

	// Assert
	requireScreen(t, after.View().Content, "the editor exited with an error", "Edit pull request")
}

func TestTheEditorShowsItIsSaving(t *testing.T) {
	t.Parallel()

	// Arrange
	// The save is held open so its in-flight state can be seen, and a key pressed
	// then is ignored rather than starting a second save.
	editing := newWorld()
	editing.editGate = make(chan struct{})
	model := editing.live(t, 120, 40)

	// Act
	saving := typing(t, model, "4", "e", keyEnter, "x")

	// Assert
	requireScreen(t, saving.View().Content, "saving")

	close(editing.editGate)
}

func TestEditingWithoutAnEditorCannotOpenTheBody(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no editor seam, ctrl+o does nothing rather than crash.
	deps := newWorld().deps()
	deps.Editor = tui.EditorDeps{}
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)
	editing := typing(t, drain(t, model, model.Init()), "4", "e")

	// Act
	after, cmd := pressed(t, editing, "ctrl+o")

	// Assert
	if cmd != nil || after.View().Content != editing.View().Content {
		t.Errorf("ctrl+o did something with no editor:\n%s", after.View().Content)
	}
}

func TestADryRunEditSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	previewed := typing(t, model, "4", "e", keyEnter)

	// Assert
	requireScreen(t, previewed.View().Content, "dry run: would save #42 as", pullTitle)

	if calls := dry.asked("edit"); len(calls) != 0 {
		t.Errorf("a dry run edited: %q", calls)
	}
}
