// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// sortTasksKey is the Tasks pane's key that cycles its order.
const sortTasksKey = "O"

func TestOSortsTheTasksWithinTheirGroupsAndSaysHow(t *testing.T) {
	t.Parallel()

	// Arrange
	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	// Act
	// The orders cycle from most urgent first: by state, then by id.
	view := typing(t, tasks, sortTasksKey, sortTasksKey).View().Content

	// Assert
	// Task 3 comes before task 12 by id, though 12 is more urgent, and the
	// groups keep their heading.
	requireInOrder(t, view, "by id", "  3 "+secondIssue, " 12 "+issueKey, "Other", "  9 Renew the cert")
}

func TestTheTasksOrderLastsThroughARefresh(t *testing.T) {
	t.Parallel()

	// Arrange
	sorted := typing(t, withTasks().live(t, 120, 40), tasksPane, sortTasksKey, sortTasksKey)

	// Act
	view := typing(t, sorted, "r").View().Content

	// Assert
	requireInOrder(t, view, "by id", "  3 "+secondIssue, " 12 "+issueKey)
}

func TestSortingKeepsTheCursorOnItsTask(t *testing.T) {
	t.Parallel()

	// Arrange
	// The cursor starts on task 12, the most urgent; by id it is second.
	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	// Act
	view := typing(t, tasks, sortTasksKey, sortTasksKey).View().Content

	// Assert
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
}

func TestSortingByPriorityShowsEachTasksPriority(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Priority = "H" })
	tasks := typing(t, repo.live(t, 140, 40), tasksPane)

	// Act
	// Most urgent, state, id, tag, issue, then priority: five presses.
	view := typing(t, tasks, sortTasksKey, sortTasksKey, sortTasksKey, sortTasksKey, sortTasksKey).View().Content

	// Assert
	requireInOrder(t, view, "by priority", "  3 "+secondIssue+": Add retries  "+secondIssue+" · priority H",
		" 12 "+issueKey, "no priority")
}

func TestTheTaskDetailNamesItsPriority(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Priority = "M" })

	// Act
	view := typing(t, repo.live(t, 140, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "workflow · priority M")
}

func TestSortingByTagNamesAnUntaggedTaskAsTheFilterDoes(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	changeTask(repo, looseTaskUUID, func(task *taskwarrior.Task) { task.Tags = nil })
	tasks := typing(t, repo.live(t, 140, 40), tasksPane)

	// Act
	// Most urgent, state, id, then tag: three presses.
	view := typing(t, tasks, sortTasksKey, sortTasksKey, sortTasksKey).View().Content

	// Assert
	requireInOrder(t, view, "by tag", "Renew the cert", "no tag")
	refuseScreen(t, view, "no tags")
}

func TestSortingByIssueAddsNothingToARow(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 3 has a priority and a tag, either of which a row names when the
	// list is sorted by it.
	repo := withTasks()
	changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Priority, task.Tags = "H", []string{"api"} })
	tasks := typing(t, repo.live(t, 140, 40), tasksPane)

	// Act
	// Most urgent, state, id, tag, then issue: four presses.
	view := typing(t, tasks, sortTasksKey, sortTasksKey, sortTasksKey, sortTasksKey).View().Content

	// Assert
	requireScreen(t, view, "by issue", "  3 "+secondIssue+": Add retries  "+secondIssue+" · 8.1")
}
