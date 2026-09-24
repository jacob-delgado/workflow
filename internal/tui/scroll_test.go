// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

// Over-scrolling must not strand the offset past the end: one scroll-up has to
// move the view, not spend itself undoing scrolls the content never had room
// for.
func TestOverScrollingStillMovesOnTheWayBack(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"
	overScrolled := typing(t, wordy.live(t, 120, 30),
		"pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "pgdown",
		"pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "pgdown")

	// Act
	back := typing(t, overScrolled, "pgup")

	// Assert
	requireScreen(t, overScrolled.View().Content, "THE END")
	refuseScreen(t, back.View().Content, "THE END")
}

// A taller terminal leaves an offset that was the end past the new end. The
// first scroll-up must move the view from where it is drawn, not spend itself
// undoing lines the taller pane no longer needs to hide.
func TestAScrollLeftPastTheEndByATallerTerminalMovesOnTheFirstKey(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line of the description\n", 60) + "THE END"
	atTheEnd := typing(t, wordy.live(t, 120, 30),
		"pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "pgdown",
		"pgdown", "pgdown", "pgdown", "pgdown", "pgdown", "pgdown")
	taller := sized(t, atTheEnd, 120, 60)

	// Act
	back := typing(t, taller, "pgup")

	// Assert
	requireScreen(t, atTheEnd.View().Content, "THE END")
	refuseScreen(t, taller.View().Content, detailTop)
	requireScreen(t, back.View().Content, detailTop)
}
