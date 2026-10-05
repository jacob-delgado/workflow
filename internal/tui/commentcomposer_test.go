// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// The comment composer's mode lines.
const (
	normalMode = "-- NORMAL --"
	insertMode = "-- INSERT --"
)

// writing opens the composer on the selected issue and types text in insert
// mode, leaving it in insert mode.
func writing(t *testing.T, model tui.Model, text string) tui.Model {
	t.Helper()

	return typing(t, model, append([]string{"c", "i"}, letters(text)...)...)
}

func TestCOpensTheComposerInNormalModeWhereLettersDoNotType(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	view := typing(t, model, "c", "x", "q").View().Content

	// Assert
	// q would quit from the pane; here the composer has the keyboard, and in
	// normal mode a letter that is not a command does nothing at all.
	requireScreen(t, view, "┏━ Comment on "+issueKey, normalMode, "i insert")
	refuseScreen(t, view, "xq")
}

func TestIEntersInsertModeWhereEveryKeyTypes(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	view := writing(t, model, "quit 7 jk").View().Content

	// Assert
	requireScreen(t, view, insertMode, "quit 7 jk", "esc normal mode")
}

func TestEscLeavesInsertModeThenClosesKeepingTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	written := writing(t, newWorld().live(t, 120, 40), "half a thought")

	// Act: esc once, back to normal mode
	normal := typing(t, written, keyEsc)

	// Assert: still composing, in normal mode
	requireScreen(t, normal.View().Content, normalMode, "half a thought")

	// Act: esc again, out of the composer
	closed := typing(t, normal, keyEsc)

	// Assert: the pane is back, and it says the draft is kept
	refuseScreen(t, closed.View().Content, "┏━ Comment on")
	requireScreen(t, closed.View().Content, "draft kept for "+issueKey)

	// Act: c again
	reopened := typing(t, closed, "c")

	// Assert: the draft is there to finish
	requireScreen(t, reopened.View().Content, normalMode, "half a thought")
}

func TestEnterInNormalModePreviewsThenPosts(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	normal := typing(t, writing(t, repo.live(t, 120, 40), "Patch up shortly"), keyEsc)

	// Act: preview it
	preview := typing(t, normal, keyEnter)

	// Assert: it is shown for a last look, and not yet posted
	requireScreen(t, preview.View().Content, "┏━ Comment on "+issueKey, "Patch up shortly", "enter post", "esc back")

	if posted := repo.asked("comment"); len(posted) != 0 {
		t.Errorf("posted before the preview was confirmed: %q", posted)
	}

	// Act: post it
	posted := typing(t, preview, keyEnter)

	// Assert: posted once, as written, and the draft is gone with it
	if calls := repo.asked("comment " + issueKey + " Patch up shortly"); len(calls) != 1 {
		t.Errorf("comment calls = %q, want one", repo.asked("comment"))
	}

	requireScreen(t, typing(t, posted, "c").View().Content, normalMode, "press i to write")
}

func TestEscInThePreviewGoesBackToTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	preview := typing(t, writing(t, newWorld().live(t, 120, 40), "Needs a word"), keyEsc, keyEnter)

	// Act
	view := typing(t, preview, keyEsc).View().Content

	// Assert
	requireScreen(t, view, normalMode, "Needs a word")
}

func TestAnEmptyCommentIsNotPreviewed(t *testing.T) {
	t.Parallel()

	// Arrange
	composer := typing(t, newWorld().live(t, 120, 40), "c")

	// Act
	view := typing(t, composer, keyEnter).View().Content

	// Assert
	requireScreen(t, view, normalMode, "nothing to post")
}

func TestOOpensALineBelowAndAAppendsAtItsEnd(t *testing.T) {
	t.Parallel()

	// Arrange
	normal := typing(t, writing(t, newWorld().live(t, 120, 40), "first"), keyEsc)

	// Act
	// o opens a line below and types there; esc, then A appends to that line.
	view := typing(t, normal, append(append([]string{"o"}, letters("second")...), keyEsc, "A", "!")...).View().Content

	// Assert
	requireTextRows(t, view, "first\nsecond!")
}

func TestCtrlOHandsTheDraftToTheEditorAndBack(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.edited = "Rewritten in my editor"
	normal := typing(t, writing(t, repo.live(t, 120, 40), "rough"), keyEsc)

	// Act
	view := typing(t, normal, keyCtrlO).View().Content

	// Assert
	requireScreen(t, view, normalMode, "Rewritten in my editor")
}

func TestADraftBelongsToItsIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := typing(t, writing(t, newWorld().live(t, 120, 40), "for the first"), keyEsc, keyEsc)

	// Act
	view := typing(t, kept, "down", "c").View().Content

	// Assert
	requireScreen(t, view, "Comment on "+secondIssue, "press i to write")
	refuseScreen(t, view, "for the first")
}

func TestAPasteIsTypedIntoTheComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	composer := typing(t, newWorld().live(t, 120, 40), "c", "i")

	// Act
	view := pasting(t, composer, "line one\r\nline two").View().Content

	// Assert
	requireTextRows(t, view, "line one\nline two")
}

func TestLStopsOnTheLastCharacterAsVimDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	// esc leaves the cursor on b; l cannot go past it, so i types before it.
	written := typing(t, writing(t, newWorld().live(t, 120, 40), "ab"), keyEsc, "l", "l")

	// Act
	view := typing(t, written, append([]string{"i"}, letters("X")...)...).View().Content

	// Assert
	requireScreen(t, view, "aXb")
}

func TestAPasteClearsAnEditorFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.editErr = errEditorFailed
	failed := typing(t, world.live(t, 120, 40), "c", keyCtrlO)

	// Act
	view := pasting(t, failed, "pasted instead").View().Content

	// Assert
	requireScreen(t, view, "pasted instead")
	refuseScreen(t, view, "the editor exited with an error")
}

func TestAPinnedFailureLeavesRoomForTheMarkupLine(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failure wraps over several rows; each must be counted, or the box
	// grows into the markup line beneath it and the frame clips that line.
	world := newWorld()
	world.cfg.Jira.MarkdownComments = false
	world.editErr = errLongReason
	composer := typing(t, world.live(t, 120, 40), "c")

	// Act
	view := typing(t, composer, keyCtrlO).View().Content

	// Assert
	requireScreen(t, view, outcomeTail, "Jira's own markup")
}
