// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A refresh of the work tree's status can fail where an earlier read
// succeeded: a corrupt index, say. The Commits pane keeps the files it listed
// and says the refresh failed, but offers nothing that writes from a list it
// can no longer vouch for.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// failedStatusRefresh is the Commits pane listing repo's work tree, then
// refreshed once git can no longer read the index.
func failedStatusRefresh(t *testing.T, repo *world) tui.Model {
	t.Helper()

	listed := typing(t, repo.live(t, 120, 40), "3")
	repo.changes, repo.changesErr = nil, errIndexCorrupt

	return typing(t, listed, "r")
}

// stagedAndUnpushed is a work tree with files staged and left to stage, on a
// branch with a commit not yet pushed: every write the Commits pane has is
// offered on it.
func stagedAndUnpushed() *world {
	repo := unpushedWorld()
	repo.changes = workTree()

	return repo
}

func TestAFailedRefreshKeepsTheWorkTreeListed(t *testing.T) {
	t.Parallel()

	// Act
	view := failedStatusRefresh(t, stagedAndUnpushed()).View().Content

	// Assert
	requireScreen(t, view, redactPath, untrackedNotes, "3 of 6 staged", "status failed", errIndexCorrupt.Error())
}

func TestAFailedRefreshOffersNoWriteFromTheWorkTreeListed(t *testing.T) {
	t.Parallel()

	// Act
	footer := footerLine(failedStatusRefresh(t, stagedAndUnpushed()).View().Content)

	// Assert
	requireScreen(t, footer, "r refresh")
	refuseScreen(t, footer, "space stage", "a stage all", "U unstage all", "x discard", "c commit", "A amend", "f fix up")
}

func TestAFailedRefreshWritesNothingFromTheWorkTreeListed(t *testing.T) {
	t.Parallel()

	// Each case is named for the key and the write the footer offers it as.
	keys := map[string]string{
		"space stage/unstage": keySpace,
		"a stage all":         "a",
		"U unstage all":       "U",
		"x discard":           "x",
		"c commit":            "c",
		"A amend":             "A",
		"f fix up":            "f",
	}
	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := stagedAndUnpushed()
			failed := failedStatusRefresh(t, repo)
			before := len(repo.asked(""))

			// Act
			view := typing(t, failed, key).View().Content

			// Assert
			if calls := repo.asked("")[before:]; len(calls) != 0 {
				t.Errorf("the key asked %q of a list a failed refresh left", calls)
			}

			refuseScreen(t, view, "┏━ Commit ", "Discard changes", "Amend the last commit", "into which commit?")
		})
	}
}

func TestAFailedRefreshDoesNotSayNothingIsStaged(t *testing.T) {
	t.Parallel()

	// Arrange
	// The list held stages nothing, but the index may have changed since.
	repo := newWorld()
	repo.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}}
	failed := failedStatusRefresh(t, repo)

	// Act
	view := typing(t, failed, "c").View().Content

	// Assert
	refuseScreen(t, view, "nothing is staged")
}
