// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The work tree's status and the selected file's diff are read again after
// every stage and every refresh, so two reads of either can be out at once.
// The Commits pane shows what the read begun last found.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// untrackedNotesTree is a work tree of one file not yet tracked.
func untrackedNotesTree() *world {
	repo := newWorld()
	repo.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}}
	repo.diff = []string{"@@ -0,0 +1 @@", "+as it was before it was staged"}

	return repo
}

func TestADiffReadBeforeAStageAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the file's diff, slowly; the file is then staged, and
	// the diff read after the stage answers first.
	repo := untrackedNotesTree()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), "3"), repo.counting("diff "), "r")
	repo.diff = []string{"@@ -0,0 +1 @@", "+as staged"}
	repo.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: 'A', Unstaged: ' '}}
	staged := typing(t, slow, keySpace)

	// Act
	view := handedOver(t, staged, late).View().Content

	// Assert
	requireScreen(t, view, "+as staged")
	refuseScreen(t, view, "as it was before it was staged")
}

func TestAnOlderStatusAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the status, slowly; the file is then staged in a shell,
	// and a second refresh finds it staged and answers first.
	repo := untrackedNotesTree()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), "3"), repo.counting("changes"), "r")
	repo.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: 'A', Unstaged: ' '}}
	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, "1 of 1 staged")
}
