// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

func TestARefusedPullRequestAfterItsPushIsPinnedInItsComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := withoutPull()
	refusing.branch.Upstream = ""
	refusing.openErr = errAlreadyOpen

	// Act
	view := typing(t, refusing.live(t, 120, 40), "4", "n", keyEnter, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "┏━ Open pull request", "✗ the forge rejected the request", "Title")

	if len(refusing.asked("push ")) != 1 || len(refusing.asked("open ")) != 1 {
		t.Errorf("calls = %q, want one push and then one pull request asked for", refusing.calls)
	}
}

func TestAnEditorFailingAfterARefusalSaysTheEditorsReasonAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := withoutPull()
	refusing.openErr, refusing.editErr = errAlreadyOpen, errEditorFailed
	refused := typing(t, refusing.live(t, 120, 40), "4", "n", keyEnter, keyEnter)
	requireScreen(t, refused.View().Content, "✗ the forge rejected the request")

	// Act
	view := typing(t, refused, "ctrl+o").View().Content

	// Assert
	requireScreen(t, view, "┏━ Open pull request", "✗ the editor exited with an error")
	refuseScreen(t, view, "the forge rejected the request")
}

func TestAMissingTitleSaysSoInPlaceOfTheForgesRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := withoutPull()
	refusing.openErr = errAlreadyOpen
	refused := typing(t, refusing.live(t, 120, 40), "4", "n", keyEnter, keyEnter)
	requireScreen(t, refused.View().Content, "✗ the forge rejected the request")

	// Act
	view := typing(t, refused, keyClearLine, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "┏━ Open pull request", "a title is required")
	refuseScreen(t, view, "the forge rejected the request")
}
