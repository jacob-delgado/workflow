// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A failed post's error belongs to the branch it was written on. Switching to
// another branch must leave it behind rather than show it against work it never
// concerned.

import "testing"

func TestABranchChangeClearsAStaleSlackError(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refused post leaves its error in the Slack pane.
	refusing := newWorld()
	refusing.postErr = errNotInChannel
	refused := typing(t, refusing.live(t, 120, 40), "5", "p", keyEnter, keyEsc)
	requireScreen(t, refused.View().Content, "state  ✗ the credential was not accepted")

	// Act
	// The checked-out branch changes under the tool; a refresh reads the new one.
	refusing.branch.Name = "fix/PROJ-388-add-retries"
	switched := typing(t, refused, "2", "r", "5")

	// Assert
	// The error concerned the branch just left, so the new branch's pane is clear.
	refuseScreen(t, switched.View().Content, "✗ the credential was not accepted")
}
