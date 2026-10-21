// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

func TestTheMonthBeforeTheDemosDayNeverMovesTheWorkedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := servedWeb(t)

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
		if ref == string(worked) {
			t.Errorf("the month moved %s, which opens the demo's day at To Do; moved %v", ref, moved)
		}
	}
}
