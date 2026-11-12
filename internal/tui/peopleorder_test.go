// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// People and groups reads Slack's directory as it opens, and again on a
// refresh, which may be asked for before the first read answers. What it
// shows is what the read begun last found.

import (
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
