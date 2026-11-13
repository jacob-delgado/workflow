// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// startedTwelveWaits is how the screen says task 12 was started, which it no
// longer says once the task waits.
const startedTwelveWaits = "started 1h12m ago"

// waitTheStartedTask hides task 12, started on PROJ-412, until two days after
// testNow: Taskwarrior still counts it started, while every display shows it
// waiting.
func waitTheStartedTask(repo *world) {
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Wait = testNow().Add(48 * time.Hour) })
}

func TestAnIssueWhoseStartedTaskWaitsIsMarkedTrackedNotActive(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view, "◐○ "+issueKey)
	refuseScreen(t, view, "◐◐ "+issueKey)
}

func TestTheSpineShowsNoRunningTaskWhileTheStartedOneWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	spine := spineLine(plain(repo.live(t, 120, 40).View().Content))

	// Assert
	refuseScreen(t, spine, "1h12m", issueKey)
}

func TestTheIssueDetailShowsAStartedTaskThatWaitsAsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view, tasksTitle, "○ #12 "+issueKey+": "+issueSummary)
	refuseScreen(t, view, "◐ #12", startedTwelveWaits)
}

func TestTheWaitingTasksShowAStartedTaskThatWaitsAsNotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 heads the waiting tasks, those for a listed issue coming first, so
	// the detail is its own.
	repo := withTasks()
	waitTheStartedTask(repo)
	narrowing := typing(t, repo.live(t, 120, 40), tasksPane, narrowTasksKey)

	// Act
	view := typing(t, narrowing, "down", keySpace, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "▸ ○  12 "+issueKey, "Water the plants")
	refuseScreen(t, view, "◐  12", startedTwelveWaits)
}

func TestASwitchStillStopsTheStartedTaskThatWaitsAndSaysItWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taskwarrior still counts task 12 started, its clock running, so a switch
	// stops it as it would any other; the look says why no list shows it.
	repo := withTasks()
	waitTheStartedTask(repo)

	switching := loopMoments()["switching branches"]
	switching.arrange(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-388's branch while task 12 is started and waits
	offered := typing(t, model, switching.keys...)

	// Assert: the offer names task 12 and its wait, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+" (waiting until 2026-09-18) and start task 3")
	requireTaskWrites(t, repo)

	// Act: switch
	typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, then task 3 started
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID, "task start "+trackedTaskUUID)
}

func TestASwitchToAnUntrackedIssueStillStopsTheStartedTaskThatWaitsAndSaysItWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	// PROJ-500 is tracked by no task, so the switch stops task 12 first and then
	// tracks PROJ-500; the look says why no list shows task 12.
	repo := withTasks()
	waitTheStartedTask(repo)
	keys := switchToTheUntrackedIssue(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-500's branch while task 12 is started and waits
	offered := typing(t, model, keys...)

	// Assert: the offer names task 12 and its wait, and nothing is written yet
	// (the look wraps before PROJ-500)
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+" (waiting until 2026-09-18), then track and start")
	requireTaskWrites(t, repo)

	// Act: switch
	typing(t, offered, keyEnter)

	// Assert: task 12 is stopped before PROJ-500 is tracked
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
}
