// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// A click below a wrapped jobs line must skip every row the line took, not one.
// This reaches the run overlay directly to set a jobs line long enough to wrap.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
)

func TestClickingBelowWrappedJobsSelectsTheRightFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	model := New(config.Config{}, nil, Deps{})
	model.width, model.height = 60, 40

	headers := []string{
		"┃ verylongjobname-alpha ❯", "┃ verylongjobname-bravo ❯",
		"┃ verylongjobname-charlie ❯", "┃ verylongjobname-delta ❯",
	}
	run := commandRun{
		marks: model.marks, styles: model.styles, title: "pre-commit", done: true,
		lines:    headers,
		jobList:  hooks.Jobs(headers),
		failures: []hooks.Location{{File: "a.go", Line: 1}, {File: "b.go", Line: 2}, {File: "c.go", Line: 3}},
	}

	jobsRows := rowsIn(wrap(run.jobs(), model.detailWidth()))
	if jobsRows < 2 {
		t.Fatalf("setup: the jobs line did not wrap (rows=%d, width=%d)", jobsRows, model.detailWidth())
	}

	secondFailureRow := 1 + jobsRows + 1 + 1 // state, wrapped jobs, blank, first failure, second failure

	// Act
	updated, _ := run.click(model, secondFailureRow)

	// Assert
	clicked, ok := updated.overlay.(commandRun)
	if !ok || clicked.selected != 1 {
		t.Errorf("clicking the second failure below wrapped jobs selected %d, want 1", clicked.selected)
	}
}
