// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// savedRemovingToken is what the fake records for a save removing the Jira
// token.
const savedRemovingToken = "save-settings removing [jira.token]"

// storedHeader is a Jira header the configuration read holds.
const storedHeader = "CF-Access-Client-Secret"

// withHeader is a world whose configuration holds storedHeader.
func withHeader() *world {
	repo := newWorld()
	repo.settings.Jira.Headers = map[string]config.Secret{storedHeader: "header-secret-1357"}

	return repo
}

func TestRemovingAStoredTokenAsksFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(tokenRow), removeKey)...).View().Content

	// Assert
	requireScreen(t, view, "Remove the Jira token", "cannot be undone")

	if saves := repo.asked(savedRemovingToken); len(saves) != 0 {
		t.Errorf("saved %d times before the last look was answered, want none", len(saves))
	}
}

func TestRemovingAStoredTokenWritesTheFileWithoutIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), append(toRow(tokenRow), removeKey, keyEnter)...)

	// Assert
	if saves := repo.asked(savedRemovingToken); len(saves) != 1 || repo.settings.Jira.Token != "" {
		t.Errorf("saved removing the token %d times, token kept %t; want it removed once",
			len(saves), repo.settings.Jira.Token != "")
	}
}

func TestBackingOutOfARemovalWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), append(toRow(tokenRow), removeKey, keyEsc)...).View().Content

	// Assert
	if saves := repo.asked(savedRemovingToken); len(saves) != 0 {
		t.Errorf("saved %d times, want nothing written", len(saves))
	}

	requireScreen(t, view, "Settings", "Base URL")
}

func TestATokenWithNothingStoredOffersNoRemoval(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.settings.Jira.Token = ""

	// Act
	view := typing(t, repo.live(t, 120, 40), toRow(tokenRow)...).View().Content

	// Assert
	refuseScreen(t, footerLine(view), "D remove")
}

func TestRemovingAStoredHeaderAsksThenWritesTheFileWithoutIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withHeader()
	asked := typing(t, repo.live(t, 120, 40), append(toRow(addHeaderRow), removeKey)...)
	requireScreen(t, asked.View().Content, "Remove the header "+storedHeader, "cannot be undone")

	// Act
	typing(t, asked, keyEnter)

	// Assert
	if _, kept := repo.settings.Jira.Headers[storedHeader]; kept {
		t.Errorf("the header was kept, want it removed")
	}
}

func TestARemovalWritesTheReadAndKeepsTheFormsOtherEdits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	keys := append(edited(projectRow, editedProject), slices.Repeat([]string{"k"}, projectRow-tokenRow)...)

	// Act
	view := typing(t, repo.live(t, 120, 40), append(keys, removeKey, keyEnter)...).View().Content

	// Assert
	if saves := repo.asked(savedRemovingToken); len(saves) != 1 || repo.settings.Jira.Project != readProject {
		t.Errorf("saved removing the token %d times, project %q; want the read written without the token",
			len(saves), repo.settings.Jira.Project)
	}

	requireScreen(t, view, "Settings", editedProject, "(edited)")
}

func TestARemovalWithNoOtherEditsReopensAsASaveDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	asked := typing(t, working.live(t, 120, 40), append(toRow(tokenRow), removeKey)...)

	// Act
	removed, cmd := pressedAndAnswered(t, asked, keyEnter)

	// Assert
	if !quits(cmd) || removed.Destination().Dir != apiCmd {
		t.Errorf("a removal: quit %v, destination %q; want the program ended for %s",
			quits(cmd), removed.Destination().Dir, apiCmd)
	}
}
