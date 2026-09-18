// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Refresh and the dry-run push notice, split from review_test.go.

import (
	"testing"
)

func TestRRefreshesTheReview(t *testing.T) {
	t.Parallel()

	// Arrange
	refreshing := newWorld()
	pane := typing(t, refreshing.live(t, 120, 40), "4")
	before := len(refreshing.asked("find"))

	// Act
	typing(t, pane, "r")

	// Assert
	if finds := len(refreshing.asked("find")); finds != before+1 {
		t.Errorf("looked for the pull request %d times, want once more than the %d before r", finds, before)
	}
}

func TestTheDryRunNoticeMentionsThePushForAnUnpushedBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	dry.branch.Upstream = ""
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View()

	// Assert
	requireScreen(t, view, "dry run: would push "+featureName+", then open")
}
