// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestTheDetailShowsTheWholeIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.detail = jira.IssueDetail{
		Issue:       jira.Issue{Key: issueKey},
		Reporter:    reporter,
		Description: "Tokens reach the log.",
		Assignee:    "Fred Ops",
		Labels:      []string{"backend", "urgent"},
		Components:  []string{"auth"},
		FixVersions: []string{"1.2.0"},
		Parent:      jira.LinkedIssue{Key: "PROJ-1", Summary: "Epic login"},
		Subtasks:    []jira.LinkedIssue{{Key: "PROJ-500", Summary: "Write test", Status: "To Do"}},
		IssueLinks: []jira.IssueLink{
			{Relation: "blocks", Issue: jira.LinkedIssue{Key: "PROJ-9", Summary: "Deploy", Status: "Closed"}},
		},
	}

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view,
		"assigned to Fred Ops", "labels backend, urgent", "components auth", "fix versions 1.2.0",
		"parent PROJ-1 Epic login",
		"Subtasks", "PROJ-500 Write test · To Do",
		"Links", "blocks PROJ-9 Deploy · Closed")
}
