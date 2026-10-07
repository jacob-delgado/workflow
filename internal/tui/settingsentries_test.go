// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// storedHeaderRow is the row of storedHeader in a configuration holding it
// alone; the row that adds a header follows it.
const storedHeaderRow = addHeaderRow

// otherHeader is a second stored header, listed before storedHeader.
const otherHeader = "A-Other"

// clientSecretRow is the row of the Slack client secret.
const clientSecretRow = messagingKindRow + 2

// storedHeaderValue is the value withHeader keeps for storedHeader.
func storedHeaderValue(repo *world) string {
	return repo.settings.Jira.Headers[storedHeader].Reveal()
}

func TestAStoredHeaderIsShownMasked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	stored := storedHeaderValue(repo)

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, settingsKey).View().Content

	// Assert
	requireScreen(t, view, "Header "+storedHeader, config.Redact(stored))
	refuseScreen(t, view, stored)
}

func TestAStoredHeaderTypedOverShowsAsANewHiddenValue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	stored := storedHeaderValue(repo)

	// Act
	view := typing(t, repo.live(t, 120, 40), edited(storedHeaderRow, typedSecret)...).View().Content

	// Assert
	requireScreen(t, view, "new value, hidden", "(edited)")
	refuseScreen(t, view, typedSecret, stored)
}

func TestAStoredHeaderTypedOverIsSavedWithTheTypedValue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	repo.settings.Jira.Headers[otherHeader] = "other-secret-2468"

	// Act
	typing(t, repo.live(t, 120, 40), editing(storedHeaderRow+1, typedSecret)...)

	// Assert
	if got, other := storedHeaderValue(repo), repo.settings.Jira.Headers[otherHeader]; got != typedSecret || other == "" {
		t.Errorf("saved the typed header %t, kept %s %t; want the typed one saved beside the other",
			got == typedSecret, otherHeader, other != "")
	}
}

func TestAStoredHeaderLeftEmptyIsSentBackAsItWasRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	mask := config.Redact(storedHeaderValue(repo))

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(storedHeaderRow), keyEnter, keyEnter, saveKey)...)

	// Assert
	if got := storedHeaderValue(repo); got != mask {
		t.Errorf("sent the header as %q, want its mask %q, which keeps the stored value", got, mask)
	}
}

func TestANewHeaderTheFormCannotTakeIsRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		typed []string
		want  string
	}{
		{"with no name", []string{keyEnter}, "an entry needs a name"},
		{"named as a stored one", append(letters(storedHeader), keyEnter), "an entry of that name is already listed"},
		{"with no value", append(letters("X-Team"), keyEnter, keyEnter), "an entry needs a value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			keys := append(toRow(storedHeaderRow+1), keyEnter)

			// Act
			view := typing(t, withHeader().live(t, 120, 40), append(keys, test.typed...)...).View().Content

			// Assert
			requireScreen(t, view, test.want)
			refuseScreen(t, view, "(edited)")
		})
	}
}

func TestRemovingTheOnlyPrefixSavesNone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Branch.Prefixes = map[string]string{"Spike": "research"}

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(addPrefixRow), removeKey, saveKey)...)

	// Assert
	if saves, got := repo.asked("save-settings"), repo.settings.Branch.Prefixes; len(saves) != 1 || len(got) != 0 {
		t.Errorf("saved %d times, branch.prefixes %v; want one save with none", len(saves), got)
	}
}

func TestClearingAMovedKeyPutsItBackOnItsDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.UI.Keys = map[string]string{commentAction: "C"}

	// Act
	typing(t, repo.live(t, 120, 40), editing(commentKeyRow(t), "")...)

	// Assert
	if saves, got := repo.asked("save-settings"), repo.settings.UI.Keys; len(saves) != 1 || len(got) != 0 {
		t.Errorf("saved %d times, ui.keys %v; want one save with none: comment back on its default", len(saves), got)
	}
}

func TestRemovingOneOfTwoStoredHeadersKeepsTheOther(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	repo.settings.Jira.Headers[otherHeader] = "other-secret-2468"

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(storedHeaderRow+1), removeKey, keyEnter)...)

	// Assert
	_, removed := repo.settings.Jira.Headers[storedHeader]
	if _, kept := repo.settings.Jira.Headers[otherHeader]; removed || !kept {
		t.Errorf("%s removed %t, %s kept %t; want only %s removed", storedHeader, removed, otherHeader, kept,
			storedHeader)
	}
}

func TestTheRemoveKeyOnASettingWithNothingToRemoveDoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(projectRow), removeKey)...).View().Content

	// Assert
	refuseScreen(t, view, "Remove", "(edited)")

	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times, want nothing written", len(saves))
	}
}

func TestRemovingASlackSecretSaysTheAccessTokenGoesToo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		row   int
		store func(*config.Config)
	}{
		{"the client secret", clientSecretRow, func(cfg *config.Config) { cfg.Messaging.ClientSecret = "secret-7777" }},
		{"the refresh token", clientSecretRow + 1, func(cfg *config.Config) { cfg.Messaging.RefreshToken = "xoxe-1-8888" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			test.store(&repo.settings)

			// Act
			view := typing(t, repo.live(t, 120, 40), append(toRow(test.row), removeKey)...).View().Content

			// Assert
			requireScreen(t, view, "Remove "+test.name, "access token made from it goes too")
		})
	}
}

func TestAKeyLeftEmptyOnItsDefaultIsSavedWithoutAnEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(commentKeyRow(t)), keyEnter, keyEnter, saveKey)...)

	// Assert
	if saves, got := repo.asked("save-settings"), repo.settings.UI.Keys; len(saves) != 1 || len(got) != 0 {
		t.Errorf("saved %d times, ui.keys %v; want one save with no entry: comment kept on its default",
			len(saves), got)
	}
}

func TestADryRunRemovesNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), append(toRow(tokenRow), removeKey, keyEnter)...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 || repo.settings.Jira.Token != settingsToken {
		t.Errorf("saved %d times under a dry run, token kept %t; want nothing written",
			len(saves), repo.settings.Jira.Token == settingsToken)
	}

	requireScreen(t, view, "dry run: would remove the Jira token")
}

func TestARefusedRemovalStaysInSettingsWithItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = errSettingsUnreadable

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(tokenRow), removeKey, keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "permission denied", "Base URL")
	refuseScreen(t, view, settingsToken)
}
