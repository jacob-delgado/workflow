// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// The keys that open Settings from the Repositories pane and save it.
const (
	settingsKey = "S"
	saveKey     = "ctrl+s"
)

// The rows of Settings the tests edit, counted from the first, Jira's base URL.
const (
	tokenRow          = 1
	projectRow        = 3
	subjectLimitRow   = 20
	branchTemplateRow = 22
	storeDisabledRow  = 26
)

// typedSecret is a credential typed into Settings.
const typedSecret = "typed-secret-4242"

// toRow opens Settings and moves down to row.
func toRow(row int) []string {
	return append([]string{reposKey, settingsKey}, slices.Repeat([]string{"j"}, row)...)
}

// editing opens Settings, edits row to text and saves.
func editing(row int, text string) []string {
	keys := append(toRow(row), keyEnter, "ctrl+u")

	return append(append(keys, letters(text)...), keyEnter, saveKey)
}

func TestSettingsShowsTheConfigurationWithItsCredentialsMasked(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), reposKey, settingsKey).View().Content

	// Assert
	requireScreen(t, view, "Settings", "Base URL", "https://jira.example.com", "Project", "PROJ", "****9999")
	refuseScreen(t, view, settingsToken)
}

func TestATypedSecretNeverReachesTheScreen(t *testing.T) {
	t.Parallel()

	// Arrange
	model := typing(t, newWorld().live(t, 120, 40), append(toRow(tokenRow), keyEnter)...)

	// Act: type the token
	typed := typing(t, model, letters(typedSecret)...)

	// Assert: it is not echoed
	refuseScreen(t, typed.View().Content, typedSecret)

	// Act: keep it
	kept := typing(t, typed, keyEnter)

	// Assert: the row does not show it either
	refuseScreen(t, kept.View().Content, typedSecret)
}

func TestSavingSendsTheEditAndLeavesTheStoredTokenMasked(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, "OSS")...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 1 || repo.settings.Jira.Project != "OSS" {
		t.Fatalf("saved %d times, project %q; want OSS saved once", len(saves), repo.settings.Jira.Project)
	}

	if token := repo.settings.Jira.Token.Reveal(); token != config.Redact(settingsToken) {
		t.Errorf("saved the token as %q, want it sent back masked so the stored one is kept", token)
	}

	requireScreen(t, view, "saved")
}

func TestSavingSendsATypedSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), editing(tokenRow, typedSecret)...)

	// Assert
	if token := repo.settings.Jira.Token.Reveal(); token != typedSecret {
		t.Errorf("saved the token as %q, want the one typed", config.Redact(token))
	}
}

func TestAnInvalidSettingIsRefusedBeforeItIsSaved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(branchTemplateRow, "no-key")...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times, want an invalid configuration refused", len(saves))
	}

	requireScreen(t, view, "not valid")
}

func TestACountMustBeAWholeNumber(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40),
		append(toRow(subjectLimitRow), keyEnter, "ctrl+u", "x", keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "a whole number")
}

func TestEnterOnASettingThatIsOnOrOffTurnsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(storeDisabledRow), keyEnter, saveKey)...)

	// Assert
	if !repo.settings.Store.Disabled {
		t.Errorf("store.disabled saved as false, want it turned on")
	}
}

func TestASaveOverAChangedFileSaysSoAndOffersAReload(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = config.ErrChangedOnDisk

	// Act
	view := typing(t, repo.live(t, 120, 40), editing(projectRow, "OSS")...).View().Content

	// Assert
	requireScreen(t, view, "changed after Settings read it")
	requireScreen(t, footerLine(view), "r reload")
}

func TestReloadReadsTheFileAgainInPlaceOfTheEdits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.saveSettingsErr = config.ErrChangedOnDisk
	refused := typing(t, repo.live(t, 120, 40), editing(projectRow, "OSS")...)

	// Act
	view := typing(t, refused, "r").View().Content

	// Assert
	if reads := repo.asked("read-settings"); len(reads) != 2 {
		t.Errorf("read %d times, want the file read again", len(reads))
	}

	refuseScreen(t, view, "OSS", "changed after Settings read it")
}

func TestADryRunSavesNoSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), editing(projectRow, "OSS")...).View().Content

	// Assert
	if saves := repo.asked("save-settings"); len(saves) != 0 {
		t.Errorf("saved %d times under a dry run", len(saves))
	}

	requireScreen(t, view, "dry run: would save")
}

func TestEscWithEditsDiscardsThem(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := append(toRow(projectRow), keyEnter, "ctrl+u", "O", keyEnter)

	// Act
	view := typing(t, newWorld().live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, footerLine(view), "esc discard")
}
