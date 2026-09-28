// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestAWaitingTaskIsOfferedToStartNotTracked(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 3 waits until later, and still tracks PROJ-388.
	repo := withTasksToStart()
	stopTheStartedTask(repo)
	changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Wait = testNow().Add(48 * time.Hour) })

	// Act
	view := typing(t, repo.live(t, 200, 40), append(branchTheSecondIssue(), keyEsc)...).View().Content

	// Assert
	requireScreen(t, view, startLook, "Start task 3 '"+secondIssue+": Add retries' in Taskwarrior? Its hooks run")
	refuseScreen(t, view, trackAndStart)
}

func TestAnIssueWhoseOnlyTaskIsCompletedIsOfferedToTrackAndStart(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 3, PROJ-388's only task, is done: starting it again would reopen it.
	repo := withTasksToStart()
	stopTheStartedTask(repo)
	changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Status = taskwarrior.Completed })
	repo.tasks.pending = onlyTasks(repo.tasks.pending, activeTaskUUID, looseTaskUUID, waitingTaskUUID)

	// Act
	view := typing(t, repo.live(t, 200, 40), append(branchTheSecondIssue(), keyEsc)...).View().Content

	// Assert
	requireScreen(t, view, trackAndStart+secondIssue)
	refuseScreen(t, view, startLook, switchLook)
}

func TestAnOfferChangesTheIssuesTaskNotTheStartedOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arrange func(repo *world)
		keys    []string
		want    string
	}{
		"completing it on a merge": {
			arrange: readyToMerge, keys: []string{"4", "M", keyEnter}, want: "task done " + activeTaskUUID,
		},
		"noting a pull request on it": {
			arrange: func(repo *world) { repo.pullFound = false },
			keys:    []string{"4", "n", keyEnter, keyEsc}, want: "task annotate " + activeTaskUUID + " #42 " + pullURL,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Task 3, on PROJ-388, is the one started; the branch is PROJ-412's,
			// which task 12 tracks.
			repo := withTasks()
			tt.arrange(repo)
			stopTheStartedTask(repo)
			changeTask(repo, trackedTaskUUID, func(task *taskwarrior.Task) { task.Start = testNow().Add(-time.Hour) })
			offered := typing(t, repo.live(t, 200, 40), tt.keys...)

			// Act
			typing(t, offered, keyEnter)

			// Assert
			requireTaskWrites(t, repo, tt.want)
		})
	}
}

func TestABranchTracksTheIssueItWasMadeFor(t *testing.T) {
	t.Parallel()

	// Arrange
	// The Issues pane reads its list again while PROJ-500's branch is made, and
	// PROJ-500 has left the list by the time the branch exists.
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	repo.moves = startTransitions()
	repo.branch.Base = ""
	stopTheStartedTask(repo)
	selected := typing(t, repo.live(t, 200, 40), selectTheUntrackedIssue()...)
	refreshing, refresh := pressed(t, selected, "r")
	repo.issues = repo.issues[:2]
	creating, create := pressed(t, typing(t, refreshing, "b"), keyEnter)
	moved := drain(t, creating, refresh)

	// Act
	view := typing(t, drain(t, moved, create), keyEsc).View().Content

	// Assert
	// The line is prefilled from the issue the branch was made for.
	requireScreen(t, view, trackAndStart+untrackedIssue, untrackedLine)
}

func TestTrackAndStartWithoutAPageIsHeldBackUnderDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// A tracker with no page for an issue, as the forge's issues have none here;
	// no task is started, and dry run is turned on as PROJ-500's branch is made.
	repo := withAnUntrackedIssue()
	repo.moves = startTransitions()
	repo.branch.Base = ""
	stopTheStartedTask(repo)
	deps := repo.deps()
	deps.Jira.BrowseURL = nil
	model := sized(t, tui.New(repo.cfg, nil, deps), 200, 40)
	selected := typing(t, drain(t, model, model.Init()), append(selectTheUntrackedIssue(), "b")...)
	creating, create := pressed(t, selected, keyEnter)
	tracking := typing(t, drain(t, creating.WithDryRun(), create), keyEsc)

	// Act
	view := typing(t, tracking, keyEnter).View().Content

	// Assert
	requireScreen(t, view,
		"dry run: task add jiraid:"+untrackedIssue+" jiraurl: +jira -- "+untrackedIssue+": Rotate the keys then start it")
	requireTaskWrites(t, repo)
}
