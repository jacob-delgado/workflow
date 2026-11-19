// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// startedTwelve is how the screen says task 12 was started.
const startedTwelve = "started 1h12m ago"

// waitTheStartedTask hides task 12, started on PROJ-412, until two days after
// testNow: Taskwarrior still counts it started, its clock running, and so does
// every display.
func waitTheStartedTask(repo *world) {
	changeTask(repo, activeTaskUUID, func(task *taskwarrior.Task) { task.Wait = testNow().Add(48 * time.Hour) })
}

func TestAnIssueWhoseStartedTaskWaitsIsMarkedActive(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view, "◐◐ "+issueKey)
}

func TestTheTasksPaneListsAStartedTaskThatWaitsAsStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	view := typing(t, repo.live(t, 120, 40), tasksPane).View().Content

	// Assert
	requireScreen(t, view, "◐  12", "1 waiting")
}

func TestTheSpineShowsTheRunningTaskWhileItWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	spine := spineLine(plain(repo.live(t, 120, 40).View().Content))

	// Assert
	requireScreen(t, spine, "1h12m", issueKey)
}

func TestTheIssueDetailShowsAStartedTaskThatWaitsAsStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view, tasksTitle, "◐ #12", startedTwelve)
}

func TestASwitchStopsTheStartedTaskThatWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	waitTheStartedTask(repo)

	switching := loopMoments()["switching branches"]
	switching.arrange(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-388's branch while task 12 is started and waits
	offered := typing(t, model, switching.keys...)

	// Assert: the offer names task 12, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+" and start task 3")
	requireTaskWrites(t, repo)

	// Act: switch
	typing(t, offered, keyEnter)

	// Assert: task 12 is stopped, then task 3 started
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID, "task start "+trackedTaskUUID)
}

func TestASwitchToAnUntrackedIssueStopsTheStartedTaskThatWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	// PROJ-500 is tracked by no task, so the switch stops task 12 first and then
	// tracks PROJ-500.
	repo := withTasks()
	waitTheStartedTask(repo)
	keys := switchToTheUntrackedIssue(repo)
	model := repo.live(t, 200, 40)

	// Act: switch to PROJ-500's branch while task 12 is started and waits
	offered := typing(t, model, keys...)

	// Assert: the offer names task 12, and nothing is written yet
	requireScreen(t, offered.View().Content, switchLook, stopTwelve+", then track and start")
	requireTaskWrites(t, repo)

	// Act: switch
	typing(t, offered, keyEnter)

	// Assert: task 12 is stopped before PROJ-500 is tracked
	requireTaskWrites(t, repo, "task stop "+activeTaskUUID)
}
