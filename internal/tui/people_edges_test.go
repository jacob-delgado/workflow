// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
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

func TestGroupsWithNoGroupToCheckSavesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.slack = newSlackWorld()
	empty.slack.groups = nil

	// Act
	typing(t, openPeople(t, empty), keyTab, keySpace, keyEnter)

	// Assert
	if calls := empty.asked("set-repo-groups "); len(calls) != 1 || calls[0] != "set-repo-groups " {
		t.Errorf("saves = %q, want none chosen", calls)
	}
}

func TestGroupsUnchecksOneOfSeveral(t *testing.T) {
	t.Parallel()

	// Arrange
	tagging := taggingWorld()

	// Act
	// check the pod beside the API reviewers, uncheck it again, and save
	typing(t, openPeople(t, tagging), keyTab, keySpace, keySpace, keyEnter)

	// Assert
	if calls := tagging.asked("set-repo-groups "); len(calls) != 1 || calls[0] != "set-repo-groups "+apiID {
		t.Errorf("saves = %q, want only the API reviewers", calls)
	}
}
