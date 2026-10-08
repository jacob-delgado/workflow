// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
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
	requireScreen(t, view, "search: cert", "Renew the cert")
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
	if notice := rows[len(rows)-2]; !strings.Contains(notice, "search: ce") {
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
	refuseScreen(t, view, "search: cert")
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
	requireScreen(t, view, "filtered to priority H", "Renew the cert")
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
	requireScreen(t, view, "tracked by task 12", "filter hides")
}

func TestAListOfOnlyWaitingTasksSortedSaysNoneArePending(t *testing.T) {
	t.Parallel()

	// Arrange
	// Sorting narrows nothing, so a list with only waiting tasks has none
	// pending to show, not none matching a filter.
	repo := withTasks()
	repo.tasks.pending = onlyTasks(repo.tasks.pending, waitingTaskUUID)
	tasks := typing(t, repo.live(t, 120, 40), tasksPane)

	// Act
	view := typing(t, tasks, sortTasksKey).View().Content

	// Assert
	requireScreen(t, view, "No pending tasks.")
	refuseScreen(t, view, "No task matches the filters.")
}

func TestTheDownArrowMovesDownWhileFilteringWhateverDownIsBoundTo(t *testing.T) {
	t.Parallel()

	// Arrange
	// With down moved to n, the arrow is no longer down's key, but while a
	// filter is typed it still moves the cursor down, as on Issues.
	repo := withTasks()
	repo.cfg.UI.Keys = map[string]string{downAction: "n", "up": "p"}
	filtering := typing(t, repo.live(t, 120, 40), tasksPane, filterTasksKey)

	// Act
	view := typing(t, filtering, downAction).View().Content

	// Assert
	requireScreen(t, view, "▸ ○   3 "+secondIssue)
}

func TestGoingToATrackingTaskPrefersOneTheListShows(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two tasks track the issue; the narrowing to +web hides the first and
	// shows the second, so T goes to the second rather than say one is hidden.
	repo := withTasks()
	second := taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000021", ID: 21, Description: issueKey + ": follow up",
		Status: taskwarrior.Pending, Tags: []string{"web"}, Urgency: 1, IssueKey: issueKey,
	}
	repo.tasks.pending = append(repo.tasks.pending, second)
	repo.tasks.linked = append(repo.tasks.linked, second)
	// The checklist offers started, pending, waiting, no priority, project
	// workflow, no project, +jira, then +web: seven rows down.
	narrowed := typing(t, repo.live(t, 120, 40), tasksPane, narrowTasksKey,
		downAction, downAction, downAction, downAction, downAction, downAction, downAction, keySpace, keyEnter)

	// Act
	view := typing(t, narrowed, "1", "T").View().Content

	// Assert
	requireScreen(t, view, "▸ ○  21 "+issueKey+": follow up")
	refuseScreen(t, view, "filter hides")
}

func TestTheCursorComesBackToItsTaskWhenAFilterThatHidItIsCleared(t *testing.T) {
	t.Parallel()

	// Arrange
	// On task 3, the second row, a filter that matches nothing hides every
	// task; clearing it shows the list again with the cursor where it was.
	moved := typing(t, withTasks().live(t, 120, 40), tasksPane, downAction)
	hidden := typing(t, moved, append([]string{filterTasksKey}, letters("zzz")...)...)

	// Act
	view := typing(t, hidden, keyEsc).View().Content

	// Assert
	requireScreen(t, view, "▸ ○   3 "+secondIssue)
}

// typedTasksFilter is the Tasks pane with cert typed into its filter, which is
// still taking keys.
func typedTasksFilter(t *testing.T) tui.Model {
	t.Helper()

	tasks := typing(t, withTasks().live(t, 120, 40), tasksPane)

	return typing(t, tasks, append([]string{filterTasksKey}, letters("cert")...)...)
}

func TestEnterKeepsTheTasksFilterAndHandsTheKeysBack(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := typing(t, typedTasksFilter(t), keyEnter)

	// Act
	// x types nothing once the filter is kept: it is no key of the pane's.
	view := typing(t, kept, "x").View().Content

	// Assert
	requireScreen(t, view, "Renew the cert")
	refuseScreen(t, view, "search: certx", secondIssue+": Add retries")
}

func TestTheUpArrowMovesUpWhileFilteringWhateverUpIsBoundTo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.cfg.UI.Keys = map[string]string{downAction: "n", "up": "p"}
	filtering := typing(t, repo.live(t, 120, 40), tasksPane, filterTasksKey, downAction)

	// Act
	view := typing(t, filtering, upAction).View().Content

	// Assert
	requireScreen(t, view, "▸ ◐  12 "+issueKey)
}

func TestBackspaceTakesTheLastLetterOffTheTasksFilter(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, typedTasksFilter(t), "x", keyBackspace).View().Content

	// Assert
	requireScreen(t, view, "search: cert", "Renew the cert")
	refuseScreen(t, view, "search: certx")
}

func TestLeavingTheTasksPaneDropsItsFilter(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"while it is typed": nil,
		"once it is kept":   {keyEnter},
	}

	for name, keys := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A click leaves the pane even while its filter takes every key.
			filtered := typing(t, typedTasksFilter(t), keys...)
			left := click(t, filtered, 5, screenRow(t, filtered.View().Content, "1 Issues"))

			// Act
			view := typing(t, left, tasksPane).View().Content

			// Assert
			requireScreen(t, view, "Renew the cert", secondIssue+": Add retries")
			refuseScreen(t, view, "search: cert")
		})
	}
}
