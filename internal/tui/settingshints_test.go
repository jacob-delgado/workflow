// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

// The rows of Settings whose hints and choices the tests read.
const (
	channelRow      = messagingKindRow + 5
	announcementRow = messagingKindRow + 6
	titleSourceRow  = branchTemplateRow + 3
)

func TestTheTokenHintNamesTheCommandItIsTakenFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token, repo.settings.Jira.TokenCommand = "", "pass show jira"

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "Taken from token_command: pass show jira")
}

func TestTheTokenHintNamesTheVariableItIsTakenFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token, repo.settings.Jira.TokenEnv = "", "JIRA_TOKEN"

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "Taken from token_env: JIRA_TOKEN")
}

func TestTheChannelHintSaysItIsForASlackUserToken(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), toRow(channelRow)...).View().Content

	// Assert
	requireScreen(t, view, "With a Slack user token; a webhook posts to its own channel.")
}

func TestTheAnnouncementHintNamesItsPlaceholdersAndThatItIsSlackOnly(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), toRow(announcementRow)...).View().Content

	// Assert
	requireScreen(t, view, "Slack only", "{author}", "{issue_url}", "Empty keeps the built-in message.")
}
