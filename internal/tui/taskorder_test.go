// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The tasks are read again after each write to Taskwarrior and on each
// refresh, so a read begun before a write can be out when the read after it
// answers. The Tasks pane shows the tasks as the write left them.

import (
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

func TestATaskReadBegunBeforeAStopAnsweringLateDoesNotShowItStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the tasks, slowly, while 12 is started; 12 is then
	// stopped, and the read after the stop answers first.
	repo := withTasks()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), tasksPane), repo.counting("tasks"), "r")
	repo.tasks.pending, repo.tasks.linked = slices.Clone(repo.tasks.pending), slices.Clone(repo.tasks.linked)
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Start = time.Time{} })

	stopped := typing(t, slow, "s")

	// Act
	view := handedOver(t, stopped, late).View().Content

	// Assert
	refuseScreen(t, view, "started 1h12m ago")
}

func TestARefreshWhoseReadIsDroppedIsNoLongerInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the tasks, slowly, while 12 is stopped; the read after
	// the stop answers first, so the refresh's own read says nothing.
	repo := withTasks()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), tasksPane), repo.counting("tasks"), "r")
	stopped := typing(t, slow, "s")

	// Act
	view := handedOver(t, stopped, late).View().Content

	// Assert
	refuseScreen(t, view, "Tasks ◐")
}
