// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
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
	view := typing(t, nobody.live(t, 140, 40), "5", "p", "down", "up", keySpace, "a", "z").View().Content

	// Assert
	requireScreen(t, view, "Announce to Slack", "tags  nobody")
	refuseScreen(t, view, "Code owners", "Groups")
}

func TestRelinkingAnOwnerReplacesWhatWasDecided(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	preview := typing(t, tagging.live(t, 140, 40), "5", "p")

	// Act
	// carla, linked, is now not on Slack
	typing(t, preview, "down", "x", keyEnter)

	// Assert
	if got := postedText(t, tagging); !strings.HasSuffix(got, "\ncc "+tagPod) {
		t.Errorf("posted %q, want only the pod tagged", got)
	}
}

func TestAWorkspaceWithoutUserGroupsSaysNothingOfIt(t *testing.T) {
	t.Parallel()

	// Arrange
	freeTier := taggingWorld()
	freeTier.slack.groupsErr = messaging.ErrNoUserGroups

	// Act
	view := typing(t, freeTier.live(t, 140, 40), "5", "p").View().Content

	// Assert
	requireScreen(t, view, "tags  @Carla Diaz @control-plane-pod")
	refuseScreen(t, view, "no user groups")
}

func TestTagsReadForAnEarlierPreviewTagNothingInARedOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// the ready-for-review preview closes before its tags are read, and CI
	// fails before the next one opens
	tagging := taggingWorld()
	opened, reads := openedPreview(t, tagging)
	closed := typing(t, opened, keyEsc)

	tagging.mu.Lock()
	tagging.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	tagging.mu.Unlock()

	red := typing(t, closed, "4", "r", "5", "p")

	// Act
	typing(t, drain(t, red, reads[0]), keyEnter)

	// Assert
	if got := postedText(t, tagging); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want a red CI's announcement untagged", got)
	}
}

func TestTagsReadForAnEarlierPreviewLeaveALaterOnesAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// carla is found not on Slack after all before the preview reopens
	tagging := taggingWorld()
	opened, reads := openedPreview(t, tagging)
	stale := reads[0]()
	closed := typing(t, opened, keyEsc)

	tagging.mu.Lock()
	tagging.slack.links[0] = loop.OwnerLink{Owner: ownerCarla, OnSlack: false, Slack: loop.SlackTarget{}}
	tagging.mu.Unlock()

	reopened := typing(t, closed, "p")

	// Act
	view := drain(t, reopened, func() tea.Msg { return stale }).View().Content

	// Assert
	requireScreen(t, view, "tags  @control-plane-pod")
	refuseScreen(t, view, carlaName)
}

func TestWhenCIPassesWaitsForWhomToTag(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	tagging.ciInterval = time.Millisecond
	tagging.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed}}
	opened, reads := openedPreview(t, tagging)

	// Act: w before the tags are read
	early := typing(t, opened, "w")

	// Assert: it is not offered, and holds nothing
	requireScreen(t, early.View().Content, "Announce to Slack", "reading whom to tag")
	refuseScreen(t, footerLine(early.View().Content), "when CI passes")

	// Act: w once they are
	typing(t, drain(t, early, reads[0]), "w")

	// Assert: the post waits for CI with its tags
	if got := postedText(t, tagging); !strings.HasSuffix(got, "\ncc "+tagCarla+" "+tagPod) {
		t.Errorf("posted %q, want the queued post to carry its tags", got)
	}
}

func TestATokenThatCannotReadUserGroupsStillTagsLinkedPeople(t *testing.T) {
	t.Parallel()

	// Arrange
	groupless := taggingWorld()
	groupless.slack.groupsErr = &messaging.MissingScopeError{Needed: "usergroups:read"}

	// Act: open the preview
	preview := typing(t, groupless.live(t, 140, 40), "5", "p")

	// Assert: the people are offered, the groups are not, and the scope is
	// named
	requireScreen(t, preview.View().Content, "Code owners", "→ Carla Diaz",
		"tagging groups needs the usergroups:read scope", "tags  @Carla Diaz", "a link to Slack")
	refuseScreen(t, preview.View().Content, "@api-reviewers", "this posts untagged")

	// Act: post
	typing(t, preview, keyEnter)

	// Assert: Carla is tagged, and no group
	if got := postedText(t, groupless); !strings.HasSuffix(got, "\ncc "+tagCarla) {
		t.Errorf("posted %q, want it to tag Carla alone", got)
	}
}

func TestADirectoryWithNoCredentialLeavesTaggingOut(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*slackWorld){
		"the members":     func(s *slackWorld) { s.membersErr = messaging.ErrNoCredential },
		"the user groups": func(s *slackWorld) { s.groupsErr = messaging.ErrNoCredential },
		"the workspace":   func(s *slackWorld) { s.workspaceErr = messaging.ErrNoCredential },
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tokenless := taggingWorld()
			arrange(tokenless.slack)

			// Act: open the preview
			preview := typing(t, tokenless.live(t, 140, 40), "5", "p")

			// Assert: it shows no tagging, and no failure
			refuseScreen(t, preview.View().Content, "Code owners", "Groups", "tags  ", "credential")

			// Act: post
			typing(t, preview, keyEnter)

			// Assert: the post goes, untagged
			if got := postedText(t, tokenless); strings.Contains(got, "cc ") {
				t.Errorf("posted %q, want it untagged", got)
			}
		})
	}
}
