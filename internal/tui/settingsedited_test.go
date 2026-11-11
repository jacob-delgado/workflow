// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestASettingTurnedBackAsReadIsNotEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(markdownRow), "space", "space")...).View().Content

	// Assert
	refuseScreen(t, view, "(edited)")
	requireScreen(t, footerLine(view), "esc close")
}

func TestAChoiceMovedBackAsReadIsNotEdited(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(messagingKindRow), keyRight, keyLeft)...).View().Content

	// Assert
	refuseScreen(t, view, "(edited)")
	requireScreen(t, footerLine(view), "esc close")
}
