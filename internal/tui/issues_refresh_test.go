// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Search failure and refresh, split from issues_test.go.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAFailedSearchReadsAsAFailureInTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	view := issuesScreen(t, failing(jira.ErrUnauthorized)).View().Content

	// Assert
	requireScreen(t, view, "✗ Jira did not accept the token")
	refuseScreen(t, view, "issues: the credential was not accepted")
}

func TestRefreshMarksTheIssuesTitleInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	refreshing, _ := pressed(t, model, "r")

	// Assert
	requireScreen(t, refreshing.View().Content, "1 Issues ◐")
}
