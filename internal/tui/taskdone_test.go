// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// offersAdd is how the footer offers adding a task, which it does whenever no
// write is on its way.
const offersAdd = "a add"

// confirmingDone is the Tasks pane with task 3 selected and the look at
// marking it done open, and the key that confirms it pressed: the done is on
// its way, and the command that sends it is returned unrun.
func confirmingDone(t *testing.T, model tui.Model) (tui.Model, tea.Cmd) {
	t.Helper()

	return pressed(t, typing(t, model, tasksPane, downAction, "d"), keyEnter)
}

// requireNoStartOrDone fails when the footer offers to start task 3 or mark it
// done, or offers no add: no write is on its way, so only those two are held.
func requireNoStartOrDone(t *testing.T, view string) {
	t.Helper()

	footer := footerLine(view)
	for _, verb := range []string{offersStart, offersDone} {
		if strings.Contains(footer, verb) {
			t.Errorf("the footer offers %q on the task just marked done: %q", verb, footer)
		}
	}

	if !strings.Contains(footer, offersAdd) {
		t.Errorf("the footer offers no %q once the done has answered: %q", offersAdd, footer)
	}
}

func TestATaskJustMarkedDoneOffersNeitherStartNorDoneUntilReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The read after the done is still on its way, so the pane still lists
	// the task as the read before the done had it.
	repo := withTasks()
	sending, send := confirmingDone(t, repo.live(t, 120, 40))

	// Act
	answered, _ := finish(t, sending, send)

	// Assert
	requireNoStartOrDone(t, answered.View().Content)
}

func TestAKeyOnATaskJustMarkedDoneWaitsForTheReadAfterIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// The task stays pending in Taskwarrior, as one undone in a terminal before
	// the read after the done would.
	repo := withTasks()
	sending, send := confirmingDone(t, repo.live(t, 120, 40))
	answered, reread := finish(t, sending, send)

	// Act: press d before the read after the done answers
	early := typing(t, answered, "d")

	// Assert: nothing asks to mark it done again
	refuseScreen(t, early.View().Content, completeLook)

	// Act: the read answers, still holding the task, and d is pressed
	again := typing(t, drain(t, early, reread), "d")

	// Assert: the done is asked, since a read since describes the task
	requireScreen(t, again.View().Content, completeLook, "Mark task 3 '"+secondIssue+": Add retries' done?")
}

func TestAReadBegunBeforeADoneKeepsStartAndDoneOffTheTask(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh is on its way as the done is sent; it reads Taskwarrior once a
	// task has been added there, so the pane shows when it has landed.
	repo := withTasks()
	refreshing, refresh := pressed(t, typing(t, repo.live(t, 120, 40), tasksPane, downAction), "r")
	sending, send := pressed(t, typing(t, refreshing, "d"), keyEnter)
	answered, _ := finish(t, sending, send)

	repo.tasks.pending = append(repo.tasks.pending, taskwarrior.Task{
		UUID: "5f1c2b3a-7d4e-4f60-8a9b-000000000030", ID: 30, Description: "Book the venue",
		Status: taskwarrior.Pending, Urgency: 1,
	})

	// Act
	read, _ := finish(t, answered, refresh)

	// Assert
	view := read.View().Content
	requireScreen(t, view, "Book the venue")
	requireNoStartOrDone(t, view)
}
