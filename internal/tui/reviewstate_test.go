// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// spineLine is the loop of stages drawn across the top of the screen.
func spineLine(view string) string {
	return strings.SplitN(view, "\n", 2)[0]
}

func TestTheReviewPaneShowsApprovalsAndMergeability(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.pull.Approvals = 2
	repo.pull.Mergeable = forge.MergeClean
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "4").View()

	// Assert
	requireScreen(t, view, "2 approvals", "mergeable")
}

func TestConflictsShowInTheReviewPane(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.pull.Mergeable = forge.MergeConflicts
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "4").View()

	// Assert
	requireScreen(t, view, "conflicts")
}

func TestChangesRequestedShowsInThePaneAndOnTheSpine(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI has passed, so only the requested changes keep the Review stage from
	// reading as done.
	repo := newWorld()
	repo.pull.ChangesRequested = true
	repo.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1}}
	model := repo.live(t, 120, 40)

	// Act
	view := typing(t, model, "4").View()

	// Assert
	requireScreen(t, view, "changes requested")

	if !strings.Contains(spineLine(view), "✗") {
		t.Errorf("the Review stage on the spine does not flag changes requested:\n%s", spineLine(view))
	}
}

func TestNoChangesRequestedLeavesTheReviewStageDone(t *testing.T) {
	t.Parallel()

	// Arrange
	// The default world's CI has passed and nothing is requested, so the Review
	// stage is done and the spine carries no failure mark.
	model := newWorld().live(t, 120, 40)

	// Act
	view := typing(t, model, "4").View()

	// Assert
	if strings.Contains(spineLine(view), "✗") {
		t.Errorf("the spine flags a failure with CI passed and nothing requested:\n%s", spineLine(view))
	}
}
