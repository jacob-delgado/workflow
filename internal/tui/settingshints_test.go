// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// The rows of Settings whose hints and choices the tests read.
const (
	channelRow      = messagingKindRow + 5
	announcementRow = messagingKindRow + 7
	titleSourceRow  = branchTemplateRow + 4
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

func TestAnEmptyTitleSourceShowsAsTheDefaultItMeans(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), toRow(titleSourceRow)...).View().Content

	// Assert
	requireScreen(t, view, "The branch's oldest commit (default)")
}

func TestAnEmptyServiceShowsAsSlackTheDefault(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), toRow(messagingKindRow)...).View().Content

	// Assert
	requireScreen(t, view, "Slack (default)")
}

func TestAChoiceComesBackRoundToTheDefaultItSavesEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.PullRequest.TitleSource = string(convention.TitleFromIssue)

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(titleSourceRow), keyEnter, saveKey)...)

	// Assert
	if got := repo.settings.PullRequest.TitleSource; got != "" {
		t.Errorf("saved pull_request.title_source %q, want empty: the default", got)
	}
}
