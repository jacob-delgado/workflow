// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// git can fail to read the branch for a moment — its index locked by another
// command — and answers no branch with its error. That is a read that failed,
// not a switch to a branch with no name: the branch and its pull request stay.

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestARefreshWhoseBranchReadFailsKeepsTheBranchAndItsPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := typing(t, repo.live(t, 140, 40), "2")
	repo.branchErr = errLocked

	// Act
	refreshed := typing(t, model, "r", "4").View().Content

	// Assert
	requireScreen(t, refreshed, featureName, "#42", "could not read the branch: "+errLocked.Error())
}

func TestARefreshWhoseBranchReadFailsKeepsAnAnnouncementWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, repo.live(t, 140, 40), "5", "p", "w")
	repo.branchErr = errLocked

	// Act
	refreshed := typing(t, waiting, "2", "r", "5").View().Content

	// Assert
	refuseScreen(t, refreshed, "dropped the announcement")
	requireScreen(t, refreshed, "◐ announces when CI passes")
}
