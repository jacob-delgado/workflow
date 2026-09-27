// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// quitGuardShown is the interface after q, with a post waiting for CI, so the
// guard asks before quitting would lose it.
func quitGuardShown(t *testing.T) tui.Model {
	t.Helper()

	waiting := newWorld()
	waiting.ci = []forge.CI{{State: forge.CIRunning}}
	queued := typing(t, waiting.live(t, 120, 40), "5", "p", "w")

	return typing(t, queued, "q")
}

func TestConfirmingTheQuitGuardQuits(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := quitGuardShown(t)

	// Act
	_, cmd := pressed(t, asked, keyEnter)

	// Assert
	if got, want := messageType(cmd), messageType(tea.Quit); got != want {
		t.Errorf("enter at the quit guard returned %s, want %s", got, want)
	}
}

func TestStayingAtTheQuitGuardKeepsThePostWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := quitGuardShown(t)

	// Act
	stayed := typing(t, asked, keyEsc)

	// Assert
	refuseScreen(t, stayed.View().Content, "will be lost")
	requireScreen(t, stayed.View().Content, "◐ announces when CI passes")
}

func TestAnyOtherKeyAtTheQuitGuardKeepsAsking(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := quitGuardShown(t)

	// Act
	still, cmd := pressed(t, asked, "x")

	// Assert
	requireScreen(t, still.View().Content, "An announcement is waiting for CI and will be lost")

	if cmd != nil {
		t.Errorf("x at the quit guard returned %s, want no command", messageType(cmd))
	}
}
