// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// The Tasks pane's keys that filter and narrow its list.
const (
	filterTasksKey = "/"
	narrowTasksKey = "f"
)

func TestSlashFiltersTheTasksAsYouType(t *testing.T) {
	t.Parallel()

	// Arrange
	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	// Act
	view := typing(t, tasks, append([]string{filterTasksKey}, letters("cert")...)...).View().Content

	// Assert
	requireScreen(t, view, "filter: cert", "Renew the cert")
	refuseScreen(t, view, secondIssue+": Add retries")
}

func TestTheTasksFilterIsShownOnTheNoticeRowWhileTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	// A long list scrolls its own heading away; the row above the footer keeps
	// what is being typed in view.
	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	// Act
	view := typing(t, tasks, filterTasksKey, "c", "e").View().Content

	// Assert
	rows := strings.Split(plain(view), "\n")
	if notice := rows[len(rows)-2]; !strings.Contains(notice, "filter: ce") {
		t.Errorf("the row above the footer is %q, want the filter being typed", notice)
	}
}

func TestEscClearsTheTasksFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	filtered := typing(t, withTasks().live(t, 120, 40), append([]string{tasksPane, filterTasksKey}, letters("cert")...)...)

	// Act
	view := typing(t, filtered, keyEsc).View().Content

	// Assert
	requireScreen(t, view, "Renew the cert", secondIssue+": Add retries")
	refuseScreen(t, view, "filter: cert")
}

func TestATasksFilterMatchingNothingSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	// Act
	view := typing(t, tasks, append([]string{filterTasksKey}, letters("zzz")...)...).View().Content

	// Assert
	requireScreen(t, view, "No task matches the filters.")
}

func TestANarrowingPicksTheTasksByPriority(t *testing.T) {
	t.Parallel()

	// Arrange
	// The checklist offers started, pending and waiting, then priority H.
	repo := withTasks()
	changeTask(repo, looseTaskUUID, func(task *taskwarrior.Task) { task.Priority = "H" })
	narrowing := typing(t, repo.live(t, 120, 40), tasksPane, narrowTasksKey)

	// Act
	view := typing(t, narrowing, "down", "down", "down", keySpace, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "narrowed to priority H", "Renew the cert")
	refuseScreen(t, view, secondIssue+": Add retries")
}

func TestPickingWaitingListsTheWaitingTasks(t *testing.T) {
	t.Parallel()

	// Arrange
	narrowing := typing(t, withTasks().live(t, 120, 40), tasksPane, narrowTasksKey)

	// Act
	view := typing(t, narrowing, "down", "down", keySpace, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Water the plants", "waits until")
	refuseScreen(t, view, "Renew the cert")
}

func TestATaskNarrowedOutStillTracksItsIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	// Narrowed to the waiting tasks, the pane hides task 12, but it still
	// tracks its issue: the view must not change which task a write targets.
	narrowed := typing(t, withTasks().live(t, 120, 40), tasksPane, narrowTasksKey, "down", "down", keySpace, keyEnter)

	// Act
	view := typing(t, narrowed, "1", "T").View().Content

	// Assert
	requireScreen(t, view, "tracked by task 12", "narrowing hides")
}
