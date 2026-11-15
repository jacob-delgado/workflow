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

func TestThePickerOpensOnceTheLinkIsSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	// ben is marked not on Slack, and the picker asked for before that is
	// saved opens nothing.
	marking, save := pressed(t, typing(t, onlyBen().live(t, 140, 40), "5", "p"), "x")
	refused := typing(t, marking, "a")

	// Act
	picking := typing(t, drain(t, refused, save), "a")

	// Assert
	requireScreen(t, picking.View().Content, "Link ben to Slack")
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

func TestPeopleLinksFromTheFirstChannelPostsCanGoTo(t *testing.T) {
	t.Parallel()

	// Arrange
	// no default channel, only a list of them
	ben := onlyBen()
	ben.cfg.Messaging.Channels = []string{ben.cfg.Messaging.Channel}
	ben.cfg.Messaging.Channel = ""

	// Act
	picking := typing(t, openPeople(t, ben), keyEnter)

	// Assert
	if calls := ben.asked("members "); len(calls) != 1 || calls[0] != "members "+devChannel {
		t.Errorf("member reads = %q, want %s's", calls, devChannel)
	}

	requireScreen(t, picking.View().Content, benName)
}

func TestALinkAnsweredAfterItsPreviewClosedLandsInNoneOpenedSince(t *testing.T) {
	t.Parallel()

	// Arrange
	// ben is marked not on Slack, which is refused once the preview it was
	// marked in has closed and another has opened.
	ben := withDirectories(onlyBen())
	ben.slack.linkErr = errDirectoryDown
	preview := typing(t, ben.live(t, 140, 40), "5", "p")
	marked, kept := withheldKeys(t, preview, ben.counting("link-owner "), "x")
	reopened := typing(t, marked, keyEsc, "p")

	// Act: the refusal is handed over
	answered := handedOver(t, reopened, kept)

	// Assert: the preview opened since shows none of it
	view := answered.View().Content
	requireScreen(t, view, "Announce to Slack", "? not linked")
	refuseScreen(t, view, "slack is down")

	// Act: close the preview and switch directory
	switched := switchingToWeb(t, typing(t, answered, keyEsc))

	// Assert: the link was answered, so the switch waits for nothing
	refuseScreen(t, switched, "wait for the link to Slack")
}

func TestARefusedLinksReasonGoesOnceTheNextLinkIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	// ben's link is refused; carla is then marked not on Slack, her link
	// still being saved.
	tagging := taggingWorld()
	tagging.slack.linkErr = errDirectoryDown
	refused := typing(t, tagging.live(t, 140, 40), "5", "p", "x")

	// Act
	marked, _ := withheldKeys(t, refused, tagging.counting("link-owner "), "down", "x")

	// Assert
	refuseScreen(t, marked.View().Content, "slack is down")

	if links := tagging.asked("link-owner "); len(links) != 2 {
		t.Errorf("link calls = %q, want ben's refused, then carla's", links)
	}
}
