// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/convention"
)

// The token's sources the hints name: a command and a variable.
const (
	sourceCommand  = "pass show jira"
	sourceVariable = "JIRA_TOKEN"
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
	repo.settings.Jira.Token, repo.settings.Jira.TokenCommand = "", sourceCommand

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "Taken from token_command: pass show jira")
}

func TestTheTokenHintNamesTheVariableItIsTakenFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token, repo.settings.Jira.TokenEnv = "", sourceVariable

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "Taken from token_env: JIRA_TOKEN")
}

func TestTheTokenHintNamesTheKeychainItIsTakenFromOverTheOtherSources(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token, repo.settings.Jira.Keychain = "", true
	repo.settings.Jira.TokenCommand, repo.settings.Jira.TokenEnv = sourceCommand, sourceVariable

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "Taken from your keychain, for this address.")
}

func TestTheTokenHintFollowsTheKeychainAsItIsTurnedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token = ""

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(keychainRow), "space", "k")...).View().Content

	// Assert
	requireScreen(t, view, "Taken from your keychain, for this address.")
}

func TestTheTokenHintSaysWhereATypedTokenIsKept(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token = ""

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	requireScreen(t, view, "A personal access token, kept in your keychain for this address on macOS")
}

func TestTheTokenHintSaysAStoredTokenIsUsedOverItsSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source func(*config.Config)
		want   string
	}{
		{
			"a command", func(cfg *config.Config) { cfg.Jira.TokenCommand = sourceCommand },
			"The token stored here is used over token_command: pass show jira.",
		},
		{
			"a variable", func(cfg *config.Config) { cfg.Jira.TokenEnv = sourceVariable },
			"The token stored here is used over token_env: JIRA_TOKEN.",
		},
		{
			"the keychain", func(cfg *config.Config) { cfg.Jira.Keychain = true },
			"The token stored here is used over your keychain, for this address.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			test.source(&repo.settings)

			// Act
			view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

			// Assert
			requireScreen(t, view, test.want)
			refuseScreen(t, view, settingsToken)
		})
	}
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

func TestAServiceTheFileNamesShowsInItsOwnWords(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Messaging.Kind = "teams"

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(messagingKindRow)...).View().Content

	// Assert
	requireScreen(t, view, "Microsoft Teams")
	refuseScreen(t, view, "Slack (default)")
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
