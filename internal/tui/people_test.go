// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// peopleTitle titles the overlay.
const peopleTitle = "People and groups"

// openPeople opens the overlay from the messaging pane.
func openPeople(t *testing.T, w *world) tui.Model {
	t.Helper()

	return typing(t, w.live(t, 140, 40), "5", "P")
}

func TestTheMessagingPaneOffersPeopleAndGroupsWithSlackAndAStore(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		slack   bool
		webhook bool
		want    bool
	}{
		"with a Slack user token and a store": {slack: true, webhook: false, want: true},
		"without either":                      {slack: false, webhook: false, want: false},
		// The directory is bound whatever the settings, so Settings can switch
		// to a user token while workflow runs; a webhook still cannot read it.
		"with a webhook": {slack: true, webhook: true, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			if tt.slack {
				repo.slack = newSlackWorld()
			}

			if tt.webhook {
				repo.cfg.Messaging.ClientID = ""
				repo.cfg.Messaging.WebhookURL = "https://hooks.example.com/x"
			}

			// Act
			opened := typing(t, repo.live(t, 140, 40), "5", "P").View().Content

			// Assert
			if offered := strings.Contains(footerLine(typing(t, repo.live(t, 140, 40), "5").View().Content),
				"P people and groups"); offered != tt.want {
				t.Errorf("P offered = %v, want %v", offered, tt.want)
			}

			if shown := strings.Contains(opened, peopleTitle); shown != tt.want {
				t.Errorf("overlay shown = %v, want %v", shown, tt.want)
			}
		})
	}
}

func TestPeopleListsWhoWasDecidedAndWhoWasNotAskedYet(t *testing.T) {
	t.Parallel()

	// Act
	view := openPeople(t, taggingWorld()).View().Content

	// Assert
	requireScreen(t, view, peopleTitle, "People", "Groups",
		"carla", "→ Carla Diaz", "dan", "· not on Slack", podTeam, "→ @control-plane-pod",
		"ben", "? not asked yet", "enter change", "x not on Slack", "d forget")
}

func TestPeopleChangesAnAssociationThroughThePicker(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	ben.slack.links = []loop.OwnerLink{{Owner: ownerBen, OnSlack: false, Slack: loop.SlackTarget{}}}

	// Act
	changed := typing(t, openPeople(t, ben), keyEnter, "down", keyEnter)

	// Assert
	if calls := ben.asked("link-owner "); len(calls) != 1 || calls[0] != benLinked {
		t.Fatalf("links = %q, want ben linked to Ben Ortiz", calls)
	}

	requireScreen(t, changed.View().Content, peopleTitle, "→ Ben Ortiz")
}

func TestPeopleMarksAnOwnerNotOnSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()

	// Act
	view := typing(t, openPeople(t, ben), "x").View().Content

	// Assert
	if calls := ben.asked("link-owner "); len(calls) != 1 || calls[0] != benNotOnSlack {
		t.Errorf("links = %q, want ben not on Slack", calls)
	}

	requireScreen(t, view, "· not on Slack")
}

func TestPeopleForgetsAnOwner(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	tagging.codeOwners = []string{ownerCarla}

	// Act
	// forget carla, who owns this branch's changes, then dan, who does not
	forgotten := typing(t, openPeople(t, tagging), "d", "d")

	// Assert
	// carla is asked again, and dan is gone
	if calls := tagging.asked("forget-owner "); len(calls) != 2 {
		t.Fatalf("forgets = %q, want carla and dan", calls)
	}

	requireScreen(t, forgotten.View().Content, "carla", "? not asked yet")
	refuseScreen(t, forgotten.View().Content, "dan ")
}

func TestARefusalStaysPinnedInPeople(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange func(*slackWorld)
		keys    []string
	}{
		"forgetting":     {arrange: func(s *slackWorld) { s.forgetErr = errDirectoryDown }, keys: []string{"d"}},
		"not on Slack":   {arrange: func(s *slackWorld) { s.linkErr = errDirectoryDown }, keys: []string{"x"}},
		"saving groups":  {arrange: func(s *slackWorld) { s.setErr = errDirectoryDown }, keys: []string{keyTab, keyEnter}},
		"reading people": {arrange: func(s *slackWorld) { s.linksErr = errDirectoryDown }, keys: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			refusing := taggingWorld()
			tt.arrange(refusing.slack)

			// Act
			view := typing(t, openPeople(t, refusing), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, peopleTitle, "slack is down")
		})
	}
}

func TestGroupsChoosesWhichUserGroupsTheRepositoryTags(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act: switch to the groups
	groups := typing(t, openPeople(t, tagging), keyTab)

	// Assert: the API reviewers are this repository's already
	requireScreen(t, groups.View().Content, "○ @control-plane-pod", "● @api-reviewers", "enter save",
		"r refresh directory")

	// Act: check the pod, and save
	saved := typing(t, groups, keySpace, keyEnter)

	// Assert: both are saved, in the order Slack lists them
	if calls := tagging.asked("set-repo-groups "); len(calls) != 1 || calls[0] != "set-repo-groups "+podID+","+apiID {
		t.Fatalf("saves = %q, want the pod and the API reviewers", calls)
	}

	requireScreen(t, saved.View().Content, "● @control-plane-pod", "saved the groups")
}

func TestGroupsKeepsARepositoryGroupSlackNoLongerLists(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	tagging.slack.repoGroups = []loop.SlackTarget{{ID: "S0GONE", Label: "gone"}}

	// Act
	saved := typing(t, openPeople(t, tagging), keyTab, "down", "down", keySpace, "up", "down", keyEnter)

	// Assert
	if calls := tagging.asked("set-repo-groups "); len(calls) != 1 || calls[0] != "set-repo-groups " {
		t.Errorf("saves = %q, want the gone group unchecked and nothing else", calls)
	}

	requireScreen(t, saved.View().Content, "○ @gone")
}

func TestGroupsRefreshesTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	typing(t, openPeople(t, tagging), keyTab, "r")

	// Assert
	if calls := tagging.asked("refresh-directory"); len(calls) != 1 {
		t.Errorf("refreshes = %q, want one", calls)
	}

	if calls := tagging.asked("user-groups"); len(calls) != 2 {
		t.Errorf("user group reads = %q, want one more after the refresh", calls)
	}
}

func TestGroupsSaysWhatTheTokenLacks(t *testing.T) {
	t.Parallel()

	// Arrange
	scopeless := taggingWorld()
	scopeless.slack.groupsErr = &messaging.MissingScopeError{Needed: "usergroups:read"}

	// Act
	view := typing(t, openPeople(t, scopeless), keyTab).View().Content

	// Assert
	requireScreen(t, view, "usergroups:read")
}

func TestPeopleSaysWhenNobodyWasDecided(t *testing.T) {
	t.Parallel()

	// Arrange
	nobody := newWorld()
	nobody.slack = newSlackWorld()

	// Act
	view := typing(t, openPeople(t, nobody), keyEnter, "x", "d", "down").View().Content

	// Assert
	requireScreen(t, view, "nobody decided yet")
}

func TestPeopleClosesWithEsc(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, openPeople(t, taggingWorld()), keyTab, keyTab, keyEsc).View().Content

	// Assert
	refuseScreen(t, view, peopleTitle)
}
