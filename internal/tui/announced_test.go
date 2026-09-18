// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"
)

// errHistoryUnavailable is a Slack history search that could not answer.
var errHistoryUnavailable = errors.New("slack history is unavailable")

func TestAPullRequestAnnouncedEarlierShowsAsPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	// The channel's history already holds the pull request's URL.
	restarted := newWorld()
	restarted.alreadyPosted = true

	// Act
	view := typing(t, restarted.live(t, 120, 40), "5").View()

	// Assert
	requireScreen(t, view, "state  ● posted")
	refuseScreen(t, footerLine(view), "p post")

	if calls := restarted.asked("history"); len(calls) != 1 || !strings.Contains(calls[0], pullURL) {
		t.Errorf("history calls = %q, want one search for the pull request's URL", calls)
	}

	if calls := restarted.asked("post "); len(calls) != 0 {
		t.Errorf("announced a pull request nobody asked to announce: %q", calls)
	}
}

func TestAnUnannouncedPullRequestStillOffersToPost(t *testing.T) {
	t.Parallel()

	// Arrange
	// The history search finds nothing.
	fresh := newWorld()
	fresh.alreadyPosted = false

	// Act
	view := typing(t, fresh.live(t, 120, 40), "5").View()

	// Assert
	requireScreen(t, view, "state  ○ nothing posted")
	requireScreen(t, footerLine(view), "p post to slack")
}

func TestAFailedHistorySearchLeavesTheSlackPaneAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	// The history search cannot answer.
	erroring := newWorld()
	erroring.alreadyPostedErr = errHistoryUnavailable

	// Act
	view := typing(t, erroring.live(t, 120, 40), "5").View()

	// Assert
	// A best-effort check that fails leaves the pane offering the post.
	requireScreen(t, view, "state  ○ nothing posted")
	requireScreen(t, footerLine(view), "p post to slack")
}
