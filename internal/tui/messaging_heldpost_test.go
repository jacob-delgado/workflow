// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// A waiting post that goes says where it went once CI passes, and that the
// store did not confirm it when it did not.

func TestAWaitingPostThatGoesSaysWhereItWent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	passesNow(repo)

	// Act
	view := typing(t, waiting, "r").View().Content

	// Assert
	requireScreen(t, view, "announced to "+slackChannel)
	refuseScreen(t, view, loop.UnconfirmedWarning)
}

func TestAWaitingPostTheStoreCouldNotConfirmSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	passesNow(repo)
	repo.mu.Lock()
	repo.storeErr = errConfirmLost
	repo.mu.Unlock()

	// Act
	view := typing(t, waiting, "r").View().Content

	// Assert
	requireScreen(t, view, "announced to "+slackChannel, loop.UnconfirmedWarning, errConfirmLost.Error())
}
