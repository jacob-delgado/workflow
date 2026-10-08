// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A preview can be opened over a post waiting for CI, which goes on its own
// once CI passes. The preview opened since is not the one that post was
// written in, so its answer leaves the preview as it is, and the preview does
// not post the same announcement again.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// reopenedOverAQueuedPost is a second preview open over a post w queued, and
// the CI check w started, not yet answered: CI was running when w was pressed
// and has passed by the time the check answers.
func reopenedOverAQueuedPost(t *testing.T, repo *world) (tui.Model, tea.Cmd) {
	t.Helper()

	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	queued, checkCI := pressed(t, typing(t, repo.live(t, 120, 40), "5", "p"), "w")

	return typing(t, queued, "p"), checkCI
}

func TestAPreviewOpenedOverAQueuedPostNeverPostsItTwice(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	reopened, checkCI := reopenedOverAQueuedPost(t, repo)

	// Act: CI passes, and the queued post goes
	passed := drain(t, reopened, checkCI)

	// Assert: it went once, and the preview opened since is still open
	requireScreen(t, passed.View().Content, "Announce to Slack")

	if calls := repo.asked("post "); len(calls) != 1 {
		t.Fatalf("post calls = %q, want the queued one", calls)
	}

	// Act: post from the preview
	again := typing(t, passed, keyEnter)

	// Assert: nothing more is sent, and the preview stays
	requireScreen(t, again.View().Content, "Announce to Slack")

	if calls := repo.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want only the queued one", calls)
	}

	// Act: leave the preview
	left := typing(t, again, keyEsc)

	// Assert: it closes
	refuseScreen(t, left.View().Content, "Announce to Slack")
}

func TestAPreviewPostsNothingWhileAQueuedPostIsOnItsWay(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	reopened, checkCI := reopenedOverAQueuedPost(t, repo)
	sending, _ := finish(t, reopened, checkCI)

	// Act
	view := typing(t, sending, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Announce to Slack", "an announcement is on its way")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("post calls = %q, want none but the queued one, still unsent", calls)
	}
}

func TestAQueuedPostsRefusalIsNotPinnedInAPreviewOpenedSince(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.postErr = errNotInChannel
	reopened, checkCI := reopenedOverAQueuedPost(t, repo)

	// Act
	refused := drain(t, reopened, checkCI).View().Content

	// Assert
	// The preview is open, with no refusal pinned under its title.
	requireScreen(t, refused, "┏━ Announce to Slack")
	refuseScreen(t, refused, "┃ ✗ the credential was not accepted")
}
