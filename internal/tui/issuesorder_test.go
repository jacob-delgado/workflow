// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A further page of the Issues list is read from where the list stood when it
// was asked for. A list read afresh since — a refresh, or its view left and
// come back to — has moved on, and a page of the list before it does not
// belong in it.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestAFurtherPageAnsweringAfterARefreshIsNotAddedToTheFreshList(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.pageSize = 3
	repo.issues = manyIssues(5)
	paging, late := withheldKeys(t, repo.live(t, 120, 40), repo.counting("search "), "ctrl+n")
	repo.issues = append([]jira.Issue{repo.issues[4]}, repo.issues[:4]...)
	refreshed := typing(t, paging, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	requireScreen(t, view, "showing 3 of 5")
}

func TestAFurtherPageOfAViewLeftAndComeBackToIsNotAddedToItsFreshList(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.pageSize = 3
	repo.issues = manyIssues(5)
	model := sized(t, tui.New(twoViewConfig(), nil, repo.deps()), 120, 40)
	paging, late := withheldKeys(t, drain(t, model, model.Init()), repo.counting("search "), "ctrl+n")
	repo.issues = append([]jira.Issue{repo.issues[4]}, repo.issues[:4]...)
	back := typing(t, paging, "v", "v")

	// Act
	view := handedOver(t, back, late).View().Content

	// Assert
	requireScreen(t, view, "showing 3 of 5")
}
