// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Two reads of the same issue can be in flight at once — the read the selection
// started, and the read again after a comment — and their answers can arrive in
// either order. The issue shown is the one the later read found.

import "testing"

func TestAnAnswerToASupersededReadIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	// The selection rests on the next issue, and its read is held. A comment is
	// then posted there, and the read again after it answers first; the first
	// read's answer, from before the comment, arrives last.
	repo := newWorld()
	repo.edited = shortComment
	away, rest := pressed(t, repo.live(t, 120, 40), "j")
	reading, firstRead := finish(t, away, rest)

	repo.detail.Description = "as it was before the comment"
	late := firstRead()

	repo.detail.Description = "after the comment"
	commented := typing(t, reading, "c", keyEnter)

	// Act
	updated, _ := commented.Update(late)

	// Assert
	view := updated.View().Content
	requireScreen(t, view, "after the comment")
	refuseScreen(t, view, "as it was before the comment")
}
