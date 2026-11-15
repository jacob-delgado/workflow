// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A post made now from the announcement's preview answers that preview, by
// the round it opened in, whatever the post's outcome: one that went, went
// unconfirmed or was announced first elsewhere closes it, and one refused
// keeps it open, saying why, with its keys back.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// errConfirmLost is a store that took the claim but could not confirm it.
var errConfirmLost = errors.New("the store went away")

func TestAPostThatWentAnswersItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	preview := typing(t, repo.live(t, 140, 40), "5", "p")

	// Act
	view := typing(t, preview, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "● announced to "+slackChannel, "state  ● announced")
	refuseScreen(t, view, "Announce to Slack", "announcing…")
}

func TestAPostTheStoreCouldNotConfirmAnswersItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	preview := typing(t, repo.live(t, 140, 40), "5", "p")
	repo.mu.Lock()
	repo.storeErr = errConfirmLost
	repo.mu.Unlock()

	// Act
	view := typing(t, preview, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "● announced to "+slackChannel, loop.UnconfirmedWarning)
	refuseScreen(t, view, "Announce to Slack", "announcing…")
}

func TestAPostAnnouncedElsewhereFirstAnswersItsPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	preview := typing(t, repo.live(t, 140, 40), "5", "p")
	repo.mu.Lock()
	repo.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}
	repo.mu.Unlock()

	// Act
	view := typing(t, preview, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "already announced from another session; not posted again")
	refuseScreen(t, view, "Announce to Slack", "announcing…")
}

func TestARefusedPostAnswersItsPreviewAndGivesItsKeysBack(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.postErr = errNotInChannel
	preview := typing(t, repo.live(t, 140, 40), "5", "p")

	// Act
	view := typing(t, preview, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "┏━ Announce to Slack", "✗ The messaging service refused the post")
	requireScreen(t, footerLine(view), "enter announce now", "esc")
	refuseScreen(t, view, "announcing…")
}

func TestAnEditorThatWouldNotOpenReplacesARefusalInThePreview(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.postErr = errNotInChannel
	repo.editErr = errEditorFailed
	refused := typing(t, repo.live(t, 140, 40), "5", "p", keyEnter)

	// Act
	view := typing(t, refused, "e").View().Content

	// Assert
	requireScreen(t, view, "┏━ Announce to Slack", errEditorFailed.Error())
	refuseScreen(t, view, "✗ The messaging service refused the post")
}

func TestPostingAgainClearsWhyTheEditorWouldNotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.editErr = errEditorFailed
	failedEdit := typing(t, repo.live(t, 140, 40), "5", "p", "e")
	requireScreen(t, failedEdit.View().Content, errEditorFailed.Error())

	repo.postErr = errNotInChannel

	// Act
	view := typing(t, failedEdit, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "✗ The messaging service refused the post")
	refuseScreen(t, view, errEditorFailed.Error())
}

func TestAnEditThatLandsClearsWhyTheEditorWouldNotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.editErr = errEditorFailed
	failedEdit := typing(t, repo.live(t, 140, 40), "5", "p", "e")
	requireScreen(t, failedEdit.View().Content, errEditorFailed.Error())

	repo.editErr, repo.edited = nil, "Shipping the retry fix"

	// Act
	view := typing(t, failedEdit, "e").View().Content

	// Assert
	requireScreen(t, view, "┏━ Announce to Slack", "Shipping the retry fix")
	refuseScreen(t, view, errEditorFailed.Error())
}
