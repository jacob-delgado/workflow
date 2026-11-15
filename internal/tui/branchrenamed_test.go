// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestRenamingARefusedBranchDropsGitsRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.createErr = errBranchExists
	refused := typing(t, branching.live(t, 120, 40), "b", keyEnter)

	// Act
	view := typing(t, refused, "x").View().Content

	// Assert
	requireScreen(t, view, "┏━ Start work on")
	refuseScreen(t, view, "already exists")
}

func TestRenamingAfterAFailedFetchKeepsTheOfferToBranchAnyway(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	branching.fetchErr = errFetchFailed
	failed := typing(t, branching.live(t, 120, 40), "b", keyEnter)

	// Act
	view := typing(t, failed, "x").View().Content

	// Assert
	requireScreen(t, view, "could not read from remote repository",
		"could not fetch; enter branches from what you already have")
}
