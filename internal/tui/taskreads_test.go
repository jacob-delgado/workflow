// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The tasks are read at start, on each refresh and after each write, and
// Taskwarrior answers two reads out at once in whatever order it likes. The
// Tasks pane lists what the read it began last found, and a refresh that
// fails keeps the tasks listed beside its failure.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// renamedLooseTask is how task 9 reads once it is renamed.
const renamedLooseTask = "Renew the TLS cert"

// errTaskExport is a read of the tasks Taskwarrior fails.
var errTaskExport = errors.New("task export: database is locked")

// renameLooseTask renames task 9 in Taskwarrior, leaving every answer
// already given as it was.
func renameLooseTask(repo *world) {
	repo.tasks.pending, repo.tasks.linked = slices.Clone(repo.tasks.pending), slices.Clone(repo.tasks.linked)
	changeTask(repo, looseTaskUUID, func(task *taskwarrior.Task) { task.Description = renamedLooseTask })
}

// requireTheRenamedTask fails unless view lists task 9 as it reads once
// renamed, and not as it read before.
func requireTheRenamedTask(t *testing.T, view string) {
	t.Helper()

	requireScreen(t, view, renamedLooseTask)
	refuseScreen(t, view, "Renew the cert")
}

func TestAnOlderTaskRefreshAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the tasks, slowly; task 9 is renamed, and a second
	// refresh answers first.
	repo := withTasks()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), tasksPane), repo.counting("tasks"), "r")
	renameLooseTask(repo)

	refreshed := typing(t, slow, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	requireTheRenamedTask(t, view)
}

func TestAReadAWriteBeganAnsweringAfterALaterRefreshIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is stopped, and the read after the stop is slow; task 9 is
	// renamed, and a refresh begun after the stop answers first.
	repo := withTasks()
	slow, late := withheldKeys(t, typing(t, repo.live(t, 120, 40), tasksPane), repo.counting("tasks"), "s")
	renameLooseTask(repo)

	refreshed := typing(t, slow, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	requireTheRenamedTask(t, view)
}

func TestTheFirstTaskReadAnsweringAfterARefreshIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read the interface starts with is slow; task 9 is renamed, and a
	// refresh answers first.
	repo := withTasks()
	model := sized(t, tui.New(repo.cfg, nil, repo.deps()), 120, 40)
	opened, late := withholding(t, model, model.Init(), repo.counting("tasks"))
	renameLooseTask(repo)

	refreshed := typing(t, opened, tasksPane, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	requireTheRenamedTask(t, view)
}

func TestASupersededTaskRefreshThatFailedSaysNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh fails, slowly; a second refresh answers first, and succeeds.
	repo := withTasks()
	listed := typing(t, repo.live(t, 120, 40), tasksPane)
	repo.tasks.err = errTaskExport
	slow, late := withheldKeys(t, listed, repo.counting("tasks"), "r")
	repo.tasks.err = nil
	refreshed := typing(t, slow, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	requireScreen(t, view, "Renew the cert", "3 pending")
	refuseScreen(t, view, "database is locked")
}

// failedTaskRefresh is the Tasks pane listing repo's tasks, then refreshed
// once Taskwarrior can no longer export them.
func failedTaskRefresh(t *testing.T, repo *world) tui.Model {
	t.Helper()

	listed := typing(t, repo.live(t, 120, 40), tasksPane)
	repo.tasks.err = errTaskExport

	return typing(t, listed, "r")
}

func TestAFailedTaskRefreshKeepsTheTasksListed(t *testing.T) {
	t.Parallel()

	// Act
	view := failedTaskRefresh(t, withTasks()).View().Content

	// Assert
	requireScreen(t, view, "Renew the cert", "3 pending", "database is locked")
}

func TestAFailedTaskRefreshKeepsTheTasksHeldShownBeyondThePane(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world func() *world
		keys  []string
		want  string
	}{
		"the Issues marks":        {world: withTasks, keys: []string{"1"}, want: "◐◐ " + issueKey},
		"an issue's Tasks block":  {world: withTasks, keys: []string{"1"}, want: "◐ #12 " + issueKey},
		"the spine's active task": {world: withTasks, keys: nil, want: "· 1h12m"},
		"the places they put issues in": {
			world: placesWorld, keys: []string{"1", placeKey}, want: "task active  1",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failed := failedTaskRefresh(t, tt.world())

			// Act
			view := typing(t, failed, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestAFailedTaskRefreshOffersNoTrackOnAnUntrackedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	failed := failedTaskRefresh(t, withAnUntrackedIssue())

	// Act
	view := typing(t, failed, "1", downAction, downAction).View().Content

	// Assert
	requireScreen(t, view, "Rotate the keys")
	refuseScreen(t, view, "T tracks it in Taskwarrior")
}

func TestAFailedTaskRefreshWritesNothingFromTheTasksListed(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	repo.tasks.install.SyncConfigured = true
	failed := failedTaskRefresh(t, repo)

	// Act
	typing(t, failed, "s", "d", keyEnter, "u", keyEnter, "S", keyEnter)

	// Assert
	requireTaskWrites(t, repo)
}

func TestAFailedTaskRefreshOffersNoVerbOnTheTasksListed(t *testing.T) {
	t.Parallel()

	// Act
	footer := footerLine(failedTaskRefresh(t, withTasks()).View().Content)

	// Assert
	requireScreen(t, footer, "r refresh", "/ search")
	refuseScreen(t, footer, "s stop", offersDone, "a add", "u undo")
}

func TestSyncIsNotOfferedWhileAWriteIsOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is stopped, and Taskwarrior has not answered yet.
	repo := withTasks()
	repo.tasks.install.SyncConfigured = true

	model := typing(t, repo.live(t, 200, 40), tasksPane)
	if footer := footerLine(model.View().Content); !strings.Contains(footer, "S sync") {
		t.Fatalf("the footer offers no sync before the write:\n%s", footer)
	}

	// Act
	writing, _ := withheldKeys(t, model, repo.counting("task "), "s")

	// Assert
	refuseScreen(t, footerLine(writing.View().Content), "S sync")
}
