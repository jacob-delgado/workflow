// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CI is read for the Review pane by its poll, by each refresh and on opening a
// pull request, so two reads can be out at once. The pane shows what the read
// begun last found.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestAnOlderCIReadAnsweringLateDoesNotHideANewerOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs when the pane opens and when a refresh reads it, whose answer is
	// slow; a second refresh finds it passed and answers first.
	repo := newWorld()
	repo.ci = []forge.CI{
		{State: forge.CIRunning, Total: 1, Done: 0},
		{State: forge.CIRunning, Total: 1, Done: 0},
		{State: forge.CIPassed, Total: 1, Done: 1},
	}
	pane := typing(t, repo.live(t, 120, 40), "4")
	slow, late := withheldKeys(t, pane, repo.counting("ci "), "r")
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, "passed")
	refuseScreen(t, view, "running")
}
