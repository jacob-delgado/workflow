// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// openedPeople presses P on the messaging pane and returns People and groups
// with its reads not yet answered, each read on its own: the people, the
// repository's groups, the channel's members, then the user groups.
func openedPeople(t *testing.T, w *world) (tui.Model, []tea.Cmd) {
	t.Helper()

	opened, cmd := pressed(t, typing(t, w.live(t, 140, 40), "5"), "P")

	batch, isBatch := cmd().(tea.BatchMsg)
	if !isBatch {
		t.Fatalf("opening People started %T, want its reads", cmd)
	}

	return opened, batch
}

func TestMembersReadWhileThePreviewsPickerIsOpenReachBoth(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, reads := openedPreview(t, onlyBen())
	picking := typing(t, drain(t, drain(t, opened, reads[0]), reads[2]), "a")

	// Act: the members arrive while the picker is open
	read := drain(t, picking, reads[1])

	// Assert: the picker offers them at once
	requireScreen(t, read.View().Content, "Link ben to Slack", benName)
	refuseScreen(t, read.View().Content, "still reading")

	// Act: go back, and open the picker again
	again := typing(t, read, keyEsc, "a")

	// Assert: the preview kept them too
	requireScreen(t, again.View().Content, benName)
	refuseScreen(t, again.View().Content, "still reading")
}

func TestMembersReadWhilePeoplesPickerIsOpenReachBoth(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, reads := openedPeople(t, taggingWorld())
	listed := drain(t, drain(t, drain(t, opened, reads[0]), reads[1]), reads[3])
	picking := typing(t, listed, keyEnter)

	// Act: the members arrive while carla's picker is open
	read := drain(t, picking, reads[2])

	// Assert: the picker offers them at once
	requireScreen(t, read.View().Content, "Link carla to Slack", benName)

	// Act: go back, and open the picker again
	again := typing(t, read, keyEsc, keyEnter)

	// Assert: People kept them too
	requireScreen(t, again.View().Content, benName)
	refuseScreen(t, again.View().Content, "still reading")
}

func TestALinkSavedWhileThePickerIsOpenShowsOnceItCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	// ben is marked not on Slack, and the picker opened before that is saved
	marking, save := pressed(t, typing(t, onlyBen().live(t, 140, 40), "5", "p"), "x")
	picking := typing(t, marking, "a")

	// Act
	back := typing(t, drain(t, picking, save), keyEsc)

	// Assert
	requireScreen(t, back.View().Content, "Announce to Slack", "· not on Slack")
}

func TestARefreshAnsweredWhilePeoplesPickerIsOpenReachesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	refreshing, refresh := pressed(t, typing(t, openPeople(t, taggingWorld()), keyTab), "r")
	picking := typing(t, refreshing, keyTab, keyEnter)

	// Act
	read := drain(t, picking, refresh)

	// Assert
	requireScreen(t, read.View().Content, "Link carla to Slack", benName)
	refuseScreen(t, read.View().Content, "still reading")
}
