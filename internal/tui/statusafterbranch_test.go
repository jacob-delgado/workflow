// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// startTransitions offers a backlog move and the in-progress move, in that
// order, so a test can tell "the first in-progress one" from "the first listed".
func startTransitions() []jira.Transition {
	return []jira.Transition{
		{ID: "21", Name: "Backlog", ToStatus: "Backlog", ToStatusCategory: jira.CategoryNew},
		{ID: "11", Name: "Start", ToStatus: "Doing", ToStatusCategory: jira.CategoryIndeterminate},
	}
}

func TestBranchingANotStartedIssueOffersItsInProgressStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second issue (PROJ-388) is To Do; the first is already In Progress.
	repo := newWorld()
	repo.moves = startTransitions()

	// Act
	view := typing(t, repo.live(t, 120, 40), "j", "b", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Change status", secondIssue, "▸ ◐ Start")
}

func TestBranchingAnInProgressIssueOffersNoStatusChange(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.moves = startTransitions()

	// Act
	view := typing(t, repo.live(t, 120, 40), "b", keyEnter).View().Content

	// Assert
	// The branch is made — so the absence of the picker is a real "no offer",
	// not a branch that quietly did nothing.
	requireScreen(t, view, "created and switched")
	refuseScreen(t, view, "Change status")
}
