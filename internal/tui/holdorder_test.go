// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A post held for CI is read on its beat and again by each refresh of the
// messaging pane, so two reads of it can be out at once in one round. The
// hold hears only the read asked last: one asked before it, answering late,
// read CI as it stood earlier.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// readsCIAs makes the world's CI read answer ci, or fail with err, from its
// next read on.
func readsCIAs(repo *world, ci forge.CI, err error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.ci, repo.ciErr = []forge.CI{ci}, err
}

func TestAnOlderReadOfAWaitingPostAnsweringLateDoesNotStepItBack(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the held post's CI while it runs, slowly; a check fails
	// meanwhile, and a second refresh reads that and answers first.
	repo, waiting := waitingOn(t, forge.CIRunning)
	slow, late := withheldKeys(t, waiting, repo.counting("ci "), "r")
	readsCIAs(repo, forge.CI{State: forge.CIFailed, Total: 1, Done: 1}, nil)

	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, "announces when CI passes · a check failed; waiting on a fix")
}

func TestAnOlderReadOfAWaitingPostFailingLateDoesNotHideANewerOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh's read of the held post's CI fails, slowly; a check has failed
	// meanwhile, and a second refresh reads that and answers first.
	repo, waiting := waitingOn(t, forge.CIRunning)
	readsCIAs(repo, forge.CI{}, errForgeDown)
	slow, late := withheldKeys(t, waiting, repo.counting("ci "), "r")
	readsCIAs(repo, forge.CI{State: forge.CIFailed, Total: 1, Done: 1}, nil)

	newer := typing(t, slow, "r")

	// Act
	view := handedOver(t, newer, late).View().Content

	// Assert
	requireScreen(t, view, "announces when CI passes · a check failed; waiting on a fix")
	refuseScreen(t, view, "trying again")
}
