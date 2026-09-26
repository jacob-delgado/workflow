// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestNothingStagedNamesNoKeyThatStageCanMoveOff(t *testing.T) {
	t.Parallel()

	// Arrange
	// stage moves off space, so a sentence naming space would send the user to
	// a key that no longer stages; the footer beside it offers the bound key.
	unstaged := newWorld()
	unstaged.cfg.UI.Keys = map[string]string{"stage": "x"}
	unstaged.changes = []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}}

	// Act
	view := typing(t, unstaged.live(t, 120, 40), "3", "c").View().Content

	// Assert
	row := ansi.Strip(rowShowing(view, "nothing is staged"))
	if row == "" || strings.Contains(row, "space") {
		t.Errorf("the nothing-staged notice reads %q, want it shown naming no key:\n%s", row, ansi.Strip(view))
	}
}
