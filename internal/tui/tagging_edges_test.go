// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// openedPreview presses p on the messaging pane and returns the preview with
// its reads not yet answered, each read on its own: the tags, the channel's
// members, then the user groups.
func openedPreview(t *testing.T, w *world) (tui.Model, []tea.Cmd) {
	t.Helper()

	opened, cmd := pressed(t, typing(t, w.live(t, 140, 40), "5"), "p")

	batch, isBatch := cmd().(tea.BatchMsg)
	if !isBatch {
		t.Fatalf("opening the preview started %T, want its reads", cmd)
	}

	return opened, batch
}

func TestReadsArrivingAfterThePreviewClosedChangeNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	opened, reads := openedPreview(t, tagging)
	closed := typing(t, opened, keyEsc)

	// Act
	for _, read := range reads {
		closed = drain(t, closed, read)
	}

	// Assert
	refuseScreen(t, closed.View().Content, "Announce to Slack", "Code owners")
}

func TestMembersOfAChannelNoLongerChosenAreNotOffered(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	ben.cfg.Messaging.Channels = []string{teamChannel}
	opened, reads := openedPreview(t, ben)
	moved := drain(t, typing(t, drain(t, opened, reads[0]), keyRight), reads[2])

	// Act
	// #dev's members arrive once #team-b is chosen
	late := drain(t, moved, reads[1])

	// Assert
	refuseScreen(t, typing(t, late, "a").View().Content, carlaName)
}

func TestThePickerSaysWhileTheDirectoryIsStillBeingRead(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, reads := openedPreview(t, onlyBen())
	proposed := drain(t, opened, reads[0])

	// Act
	picking := typing(t, proposed, "a")

	// Assert
	requireScreen(t, picking.View().Content, "still reading Slack's directory", "Not on Slack")
}

func TestReadingWhomToTagSaysSoAndTakesNoTagKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, _ := openedPreview(t, onlyBen())

	// Act
	view := typing(t, opened, "x", "a").View().Content

	// Assert
	requireScreen(t, view, "reading whom to tag", "Announce to Slack")
}

func TestLinkingAnotherOwnerKeepsTheGroupsAsChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	// ben is not on Slack, which leaves the pod tagged and the API
	// reviewers not
	marked := typing(t, preview, "x", keyEnter)

	// Assert
	requireScreen(t, marked.View().Content, "announced")

	if got := postedText(t, tagging); !strings.HasSuffix(got, "\ncc "+tagCarla+" "+tagPod) {
		t.Errorf("posted %q, want Carla and the pod tagged as before", got)
	}
}

func TestAGroupSlackCannotTagPostsUntagged(t *testing.T) {
	t.Parallel()

	// Arrange
	odd := onlyBen()
	odd.slack.repoGroups = []loop.SlackTarget{{ID: "not-a-group", Label: "odd"}}
	odd.slack.last, odd.slack.lastChosen = []string{"not-a-group"}, true

	// Act
	typing(t, odd.live(t, 140, 40), "5", "p", keyEnter)

	// Assert
	if got := postedText(t, odd); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want it untagged", got)
	}
}

func TestAWorkspaceWithoutUserGroupsOffersATeamOnlyNotOnSlack(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*slackWorld){
		"no user groups to read":      func(s *slackWorld) { s.noGroups = true },
		"a workspace without any":     func(s *slackWorld) { s.groupsErr = messaging.ErrNoUserGroups },
		"user groups read as no rows": func(s *slackWorld) { s.groups = nil },
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			team := newWorld()
			team.codeOwners = []string{podTeam}
			team.slack = newSlackWorld()
			arrange(team.slack)

			// Act
			view := typing(t, team.live(t, 140, 40), "5", "p", "a").View().Content

			// Assert
			requireScreen(t, view, "Link "+podTeam+" to Slack", "Not on Slack")
			refuseScreen(t, view, "@"+podName)
		})
	}
}

func TestNobodyToTagLeavesNothingToMove(t *testing.T) {
	t.Parallel()

	// Arrange
	nobody := newWorld()
	nobody.slack = newSlackWorld()

	// Act
	view := typing(t, nobody.live(t, 140, 40), "5", "p", "down", "up", keySpace, "a").View().Content

	// Assert
	requireScreen(t, view, "Announce to Slack", "tags  nobody")
	refuseScreen(t, view, "Code owners", "Groups")
}
