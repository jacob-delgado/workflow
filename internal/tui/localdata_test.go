// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// localDataKey opens Local data from the Repositories pane.
const localDataKey = "L"

// errStoreUnreadable is a store directory that cannot be read.
var errStoreUnreadable = errors.New("permission denied")

func TestLocalDataListsEachFileWithItsSizeAndWhatItHolds(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), reposKey, localDataKey).View().Content

	// Assert
	requireScreen(t, view, "Local data", "workflow.db", "92.0 KiB", "scopes: 3", "kept.db", "favorite directories: 2")
}

func TestRemovingTheCacheAsksFirstAndSaysItCannotBeUndone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c").View().Content

	// Assert
	requireScreen(t, view, "Remove workflow.db?", "cannot be undone", "enter remove")

	if removed := repo.asked("remove-local-data"); len(removed) != 0 {
		t.Errorf("removed %q before the last look", removed)
	}
}

func TestConfirmingRemovesTheCacheAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c", keyEnter).View().Content

	// Assert
	if removed := repo.asked("remove-local-data"); len(removed) != 1 || removed[0] != "remove-local-data cache" {
		t.Errorf("removed %q, want the cache removed once", removed)
	}

	requireScreen(t, view, "removed workflow.db")
}

func TestRemovingEverythingSaysThePeopleAndGroupsGoWithIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "C").View().Content

	// Assert
	requireScreen(t, view, "Remove workflow.db and kept.db?", "will be asked again")
}

func TestConfirmingRemovesEverything(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "C", keyEnter)

	// Assert
	if removed := repo.asked("remove-local-data"); len(removed) != 1 || removed[0] != "remove-local-data all" {
		t.Errorf("removed %q, want everything removed once", removed)
	}
}

func TestEscFromTheLastLookRemovesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c", keyEsc)

	// Assert
	if removed := repo.asked("remove-local-data"); len(removed) != 0 {
		t.Errorf("removed %q after esc", removed)
	}
}

func TestARefusedRemovalStaysInTheLookWithItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.removeErr = errStoreUnreadable

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "permission denied", "Remove workflow.db?")
}

func TestADryRunRemovesNoLocalData(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()).WithDryRun(), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), reposKey, localDataKey, "c", keyEnter).View().Content

	// Assert
	if removed := repo.asked("remove-local-data"); len(removed) != 0 {
		t.Errorf("removed %q under a dry run", removed)
	}

	requireScreen(t, view, "dry run: would remove workflow.db")
}

func TestNoLocalDataOffersNothingToRemove(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localFiles = nil

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey).View().Content

	// Assert
	requireScreen(t, view, "No local data: there is nothing to remove.")
	refuseScreen(t, footerLine(view), "remove")
}

func TestLocalDataThatCannotBeReadSaysSoAndOffersAnotherRead(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localDataErr = errStoreUnreadable

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey).View().Content

	// Assert
	requireScreen(t, view, "permission denied")
	requireScreen(t, footerLine(view), "r try again")
}

func TestTryingAgainReadsTheLocalDataAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localDataErr = errStoreUnreadable
	refused := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c")
	repo.mu.Lock()
	repo.localDataErr = nil
	repo.mu.Unlock()

	// Act
	view := typing(t, refused, "r").View().Content

	// Assert
	requireScreen(t, view, "workflow.db")
}

func TestEscClosesLocalData(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), reposKey, localDataKey, keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "Kept in")
}

func TestWithOnlyTheKeptFileThereIsNoCacheToRemove(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localFiles = []store.DataFile{{Name: "kept.db", Kind: store.DataKept, Bytes: 512}}

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "c").View().Content

	// Assert
	requireScreen(t, view, "512 B", "not readable as a database")
	refuseScreen(t, view, "Remove kept.db?")
}

func TestALargeFileIsSizedInMiB(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localFiles = []store.DataFile{{Name: "workflow.db", Kind: store.DataCache, Bytes: 3 << 20}}

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey).View().Content

	// Assert
	requireScreen(t, view, "3.0 MiB")
}

func TestKeysLocalDataHasNothingForChangeNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.localFiles = nil

	// Act
	view := typing(t, repo.live(t, 120, 40), reposKey, localDataKey, "r", "C", "c").View().Content

	// Assert
	if removed := repo.asked("remove-local-data"); len(removed) != 0 {
		t.Errorf("removed %q with nothing to remove", removed)
	}

	requireScreen(t, view, "No local data")
}
