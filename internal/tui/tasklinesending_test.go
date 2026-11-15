// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestATaskLineBeingSentTakesNoKey(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	typed := typing(t, repo.live(t, 120, 40), append([]string{tasksPane, "a"}, letters("Renew the cert")...)...)
	sending, _ := pressed(t, typed, keyEnter)

	// Act
	view := typing(t, sending, "x", keyEsc).View().Content

	// Assert
	requireScreen(t, view, "Add a task", "Renew the cert", "sending…")
	refuseScreen(t, view, "Renew the certx")
}
