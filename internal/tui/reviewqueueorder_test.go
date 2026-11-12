// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The review queue is read at start and again on each refresh, so two reads of
// it can be out at once. The Reviews pane lists what the read begun last
// found.

import (
	"testing"
)

func TestAnOlderReviewQueueAnsweringLateIsNotListed(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the queue, slowly, while two pull requests wait on you;
	// one is reviewed meanwhile, and a second refresh answers first.
	repo := withReviews()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), "6"), repo.counting("reviews"), "r")
	repo.reviews = repo.reviews[1:]
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, "1 waiting")
}
