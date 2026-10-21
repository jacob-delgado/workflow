// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

func TestTheMonthBeforeTheDemosDayMovesNoIssueStillToDo(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)
	notStarted := map[string]bool{}

	for _, issue := range demo.frame().Issues.Issues {
		notStarted[issue.Key] = issue.StatusCategory == api.StatusCategoryNew
	}

	// Act
	activity := read[api.Activity](demo, "/api/activity?from=2026-09-01&to=2026-09-15")

	// Assert
	var moved []string

	for _, item := range itemsOf(activity) {
		if item.Source == api.ActivitySourceNameJira && item.Verb == "moved" {
			moved = append(moved, item.Ref)
		}
	}

	if len(moved) == 0 {
		t.Fatal("the month shows no Jira issue moved, want the month's moves")
	}

	for _, ref := range moved {
		if notStarted[ref] {
			t.Errorf("the month moved %s into review, which the Issues list shows still to do; moved %v", ref, moved)
		}
	}
}
