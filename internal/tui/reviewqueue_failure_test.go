// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A refresh of the review queue can fail where an earlier read succeeded. The
// Reviews pane keeps the queue it listed, says the refresh failed, and goes on
// sorting and narrowing what it holds.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// failedReviewsRefresh is the Reviews pane listing withReviews' queue, then
// refreshed while the forge cannot be reached.
func failedReviewsRefresh(t *testing.T) string {
	t.Helper()

	repo := withReviews()
	listed := typing(t, repo.live(t, 120, 40), "6")
	repo.reviewsErr = forge.ErrUnreachable

	return typing(t, listed, "r").View().Content
}

func TestAFailedRefreshKeepsTheReviewQueueListed(t *testing.T) {
	t.Parallel()

	// Act
	view := failedReviewsRefresh(t)

	// Assert
	requireScreen(t, view, "fix flaky redaction test", "add request retries", "2 waiting", "could not reach the forge")
}

func TestAFailedRefreshLeadsTheReviewsRailWhenItHasOneRow(t *testing.T) {
	t.Parallel()

	// Arrange
	// On a short terminal, the Reviews rail out of focus keeps a single row.
	repo := withReviews()
	listed := typing(t, repo.live(t, 120, 30), "6")
	repo.reviewsErr = forge.ErrUnreachable

	// Act
	view := typing(t, listed, "r", "1").View().Content

	// Assert
	requireScreen(t, view, "could not reach the forge")
}

func TestAFailedRefreshStillSortsAndNarrowsTheQueueListed(t *testing.T) {
	t.Parallel()

	// Act
	footer := footerLine(failedReviewsRefresh(t))

	// Assert
	requireScreen(t, footer, sortOffer, filterOffer)
}
