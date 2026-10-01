// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPeopleSaysWhileItReads(t *testing.T) {
	t.Parallel()

	// Act
	opened, _ := pressed(t, typing(t, taggingWorld().live(t, 140, 40), "5"), "P")

	// Assert
	requireScreen(t, opened.View().Content, peopleTitle, "reading who was decided")
}

func TestPeopleReadsArrivingAfterItClosedChangeNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	opened, cmd := pressed(t, typing(t, taggingWorld().live(t, 140, 40), "5"), "P")

	reads, isBatch := cmd().(tea.BatchMsg)
	if !isBatch {
		t.Fatalf("opening People started %T, want its reads", cmd)
	}

	closed := typing(t, opened, keyEsc)

	// Act
	for _, read := range reads {
		closed = drain(t, closed, read)
	}

	// Assert
	refuseScreen(t, closed.View().Content, peopleTitle)
}

func TestALinkSavedAfterPeopleClosedChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	chosen, save := pressed(t, typing(t, openPeople(t, ben), keyEnter), keyEnter)
	closed := typing(t, chosen, keyEsc)

	// Act
	after := drain(t, closed, save)

	// Assert
	refuseScreen(t, after.View().Content, peopleTitle)

	if calls := ben.asked("link-owner "); len(calls) != 1 {
		t.Errorf("links = %q, want the one chosen", calls)
	}
}

func TestPeopleTakesNoKeyWhileSaving(t *testing.T) {
	t.Parallel()

	// Arrange
	ben := onlyBen()
	saving, _ := pressed(t, openPeople(t, ben), "x")

	// Act
	still := typing(t, saving, "d", keyEsc)

	// Assert
	requireScreen(t, still.View().Content, peopleTitle, "saving")
	refuseScreen(t, footerLine(still.View().Content), "forget")

	if calls := ben.asked("forget-owner "); len(calls) != 0 {
		t.Errorf("forgot %q while a save was out", calls)
	}
}

func TestRefreshingTheDirectorySaysSoAndIsDroppedOnceClosed(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()
	refreshing, refresh := pressed(t, typing(t, openPeople(t, tagging), keyTab), "r")

	// Act: look while it reads
	view := refreshing.View().Content

	// Assert: it says so
	requireScreen(t, view, "reading the user groups")

	// Act: close, then let the read answer
	after := drain(t, typing(t, refreshing, keyEsc), refresh)

	// Assert: nothing reopens
	refuseScreen(t, after.View().Content, peopleTitle)
}

func TestPeopleIsNotOfferedWithoutTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	storeOnly := taggingWorld()
	storeOnly.slack.noDirectory = true

	// Act
	view := typing(t, storeOnly.live(t, 140, 40), "5", "P").View().Content

	// Assert
	refuseScreen(t, view, peopleTitle)
}

func TestKeysPeopleDoesNotUseChangeNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	view := typing(t, openPeople(t, tagging), "z", keyTab, "z").View().Content

	// Assert
	requireScreen(t, view, peopleTitle, "@api-reviewers")

	if calls := append(tagging.asked("link-owner "), tagging.asked("set-repo-groups ")...); len(calls) != 0 {
		t.Errorf("an unused key saved %q", calls)
	}
}

func TestGroupsWithNoGroupToCheckSavesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.slack = newSlackWorld()
	empty.slack.groups = nil

	// Act
	typing(t, openPeople(t, empty), keyTab, keySpace, keyEnter)

	// Assert
	if calls := empty.asked("set-repo-groups "); len(calls) != 0 {
		t.Errorf("saves = %q, want none", calls)
	}
}

func TestGroupsUnchecksOneOfSeveral(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	// check the pod beside the API reviewers, then uncheck it again
	typing(t, openPeople(t, tagging), keyTab, keySpace, keySpace)

	// Assert
	want := []string{"set-repo-groups " + podID + "," + apiID, "set-repo-groups " + apiID}
	if calls := tagging.asked("set-repo-groups "); !slices.Equal(calls, want) {
		t.Errorf("saves = %q, want %q", calls, want)
	}
}

func TestGroupsTakeNoChangeUntilTheRepositoryGroupsAreRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// everything is read but the groups this repository already tags
	tagging := taggingWorld()
	opened, reads := openedPeople(t, tagging)
	read := drain(t, drain(t, drain(t, opened, reads[0]), reads[2]), reads[3])

	// Act
	view := typing(t, read, keyTab, keySpace, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "reading the groups this repository tags")
	refuseScreen(t, footerLine(view), "tag")

	if calls := tagging.asked("set-repo-groups "); len(calls) != 0 {
		t.Errorf("saved %q before the repository's groups were read", calls)
	}
}

func TestGroupsTakeNoChangeWhenTheRepositoryGroupsCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	unread := taggingWorld()
	unread.slack.repoErr = errDirectoryDown

	// Act
	view := typing(t, openPeople(t, unread), keyTab, keySpace).View().Content

	// Assert
	requireScreen(t, view, "slack is down")

	if calls := unread.asked("set-repo-groups "); len(calls) != 0 {
		t.Errorf("saved %q over a list that could not be read", calls)
	}
}

func TestAGroupCheckedIsKeptOnceGroupsCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	view := typing(t, openPeople(t, tagging), keyTab, keySpace, keyEsc, "P", keyTab).View().Content

	// Assert
	requireScreen(t, view, "● @control-plane-pod", "● @api-reviewers")
}

func TestARefusedGroupSaveShowsWhatIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	refusing := taggingWorld()
	refusing.slack.setErr = errDirectoryDown

	// Act
	view := typing(t, openPeople(t, refusing), keyTab, keySpace).View().Content

	// Assert
	requireScreen(t, view, "slack is down", "○ @control-plane-pod", "● @api-reviewers")
}

func TestTheSelectionFollowsAnOwnerAfterAChange(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"forgetting an owner of the changes stays on them": {
			keys: []string{"d", "x"}, want: "link-owner " + ownerCarla + " nobody",
		},
		"forgetting anyone else moves to the next": {
			keys: []string{downAction, "d", "x"}, want: "link-owner " + podTeam + " nobody",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// carla, dan and the team were decided; carla owns the changes
			tagging := taggingWorld()
			tagging.codeOwners = []string{ownerCarla}

			// Act
			typing(t, openPeople(t, tagging), tt.keys...)

			// Assert
			if calls := tagging.asked("link-owner "); len(calls) != 1 || calls[0] != tt.want {
				t.Errorf("links = %q, want %q", calls, tt.want)
			}
		})
	}
}
