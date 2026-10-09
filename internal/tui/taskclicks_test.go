// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A click on the Tasks pane's list selects the task drawn on its row, and
// nothing where no task is drawn: a heading, or a failure in the list's place.

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestClickingATaskBelowTheHeadingSelectsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Focused, the list is drawn in the detail from its top row: 12, 3, the
	// heading, then 9 on row 5.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act
	view := click(t, model, 60, 5).View().Content

	// Assert
	requireScreen(t, view, "▸ ○   9 Renew the cert")
}

func TestClickingTheHeadingKeepsTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// The heading is on row 4, below task 3; clicking it selects neither.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act
	view := click(t, model, 60, 4).View().Content

	// Assert
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
}

func TestAClickOnAFailedReadSelectsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pending list answered and the linked read failed, so the failure is
	// drawn where the list would be, task 9 on row 5 among it.
	repo := withTasks()
	repo.tasks.linkedErr = fmt.Errorf("%w: unexpected end of JSON input", taskwarrior.ErrBadOutput)
	model := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act: click where task 9 would be
	clicked := click(t, model, 60, 5)

	// Assert: the failure is still what the pane shows
	requireScreen(t, clicked.View().Content, "unreadable answer")

	// Act: read again, and this time Taskwarrior answers
	repo.tasks.linkedErr = nil
	view := typing(t, clicked, "r").View().Content

	// Assert: the click chose no task, so the first is still selected
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
	refuseScreen(t, view, "▸ ○   9")
}
