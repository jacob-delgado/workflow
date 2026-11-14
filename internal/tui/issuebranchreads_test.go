// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The Issues list marks each issue a branch names in flight, from a listing
// of the branches made at start, on a refresh and after a branch is made.
// The marks are what the listing begun last found.

import (
	"slices"
	"strings"
	"testing"
)

func TestAnOlderBranchListingAnsweringLateDoesNotTakeAnIssueOutOfFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh lists the branches, slowly; a branch for PROJ-388 is made
	// elsewhere, and a second refresh, which lists it, answers first.
	repo := newWorld()
	slow, late := withheldKeys(t, repo.live(t, 120, 40), repo.counting("branches"), "r")
	repo.branches = append(slices.Clone(repo.branches), otherTaskBranch)
	refreshed := typing(t, slow, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	if line := railLine(view, "PROJ-388"); !strings.Contains(line, "◐ PROJ-388") {
		t.Errorf("PROJ-388 is listed as %q, want it in flight:\n%s", line, plain(view))
	}
}
