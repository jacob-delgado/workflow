// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestTheWorkedIssuesTaskIsNotStartedAtTheStart(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	pending, err := deps.Tasks.Pending(t.Context())

	// Assert
	at := slices.IndexFunc(pending.Tasks, func(task taskwarrior.Task) bool { return task.IssueKey == string(worked) })
	if err != nil || at < 0 {
		t.Fatalf("the pending tasks = %d, %v; want PROJ-412's among them", len(pending.Tasks), err)
	}

	if started := pending.Tasks[at].Start; !started.IsZero() {
		t.Errorf("PROJ-412's task was started at %v, want it not started", started)
	}
}
