// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// People and groups reads Slack's directory as it opens, and again on a
// refresh, which may be asked for before the first read answers; the
// announcement preview reads a channel's members as it opens, and again each
// time the channel changes. What either shows is what the read begun last
// found.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/loop"
)

func TestTheDirectoryReadAsPeopleOpenedAnsweringAfterARefreshIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// The user groups read as People opened answer slowly; a group is made in
	// Slack meanwhile, and a refresh, which reads it, answers first.
	tagging := taggingWorld()
	opened, reads := openedPeople(t, tagging)
	slowGroups := reads[3]()
	listed := drain(t, drain(t, drain(t, opened, reads[0]), reads[1]), reads[2])

	tagging.slack.groups = append(tagging.slack.groups, loop.SlackTarget{ID: "S0NEW", Label: "release-team"})
	refreshed := typing(t, listed, keyTab, "r")

	// Act
	view := drain(t, refreshed, func() tea.Msg { return slowGroups }).View().Content

	// Assert
	requireScreen(t, view, "@release-team")
}

func TestAnEarlierRefreshOfTheDirectoryAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the user groups, slowly; a group is made in Slack
	// meanwhile, and a second refresh, which reads it, answers first.
	tagging := taggingWorld()
	refreshing, slowRefresh := pressed(t, typing(t, openPeople(t, tagging), keyTab), "r")
	slow := slowRefresh()

	tagging.slack.groups = append(tagging.slack.groups, loop.SlackTarget{ID: "S0NEW", Label: "release-team"})
	refreshed := typing(t, refreshing, "r")

	// Act
	view := drain(t, refreshed, func() tea.Msg { return slow }).View().Content

	// Assert
	requireScreen(t, view, "@release-team")
}

func TestAnEarlierReadOfAChannelChosenAgainAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// #dev's members, read as the preview opened, fail, slowly; #team-b is
	// chosen, then #dev again, whose second read answers first.
	ben := onlyBen()
	ben.cfg.Messaging.Channels = []string{teamChannel}
	ben.slack.membersErr = errDirectoryDown
	opened, reads := openedPreview(t, ben)
	failed := reads[1]()

	ben.slack.membersErr = nil
	cycled := typing(t, drain(t, drain(t, opened, reads[0]), reads[2]), keyRight, keyLeft)

	// Act
	late := drain(t, cycled, func() tea.Msg { return failed })

	// Assert
	view := typing(t, late, "a").View().Content
	requireScreen(t, view, "Link ben to Slack", benName)
	refuseScreen(t, view, "slack is down")
}

func TestAnEarlierRereadOfThePeopleAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// carla is marked not on Slack, and the people read again after that,
	// slowly; ben is marked too, and the read after his answers first.
	opened := openPeople(t, taggingWorld())
	marking, save := pressed(t, opened, "x")
	saved, reread := finish(t, marking, save)
	slow := reread()
	marked := typing(t, saved, "down", "down", "down", "x")

	// Act
	view := drain(t, marked, func() tea.Msg { return slow }).View().Content

	// Assert
	if line := lineShowing(t, view, ownerBen+" "); !strings.Contains(line, "not on Slack") {
		t.Errorf("ben is listed as %q, want him not on Slack:\n%s", line, plain(view))
	}
}
