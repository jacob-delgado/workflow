// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// How the footer offers starting and completing a task not started.
const (
	offersStart = "s start"
	offersDone  = "d mark done"
)

func TestATaskWriteWaitsForTheOneInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taskwarrior commits writes in the order they finish, so an undo sent while
	// a done is held by a hook would revert whatever came before the done.
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane, downAction)

	// Act: complete task 3, then press undo before Taskwarrior answers
	completing, done := pressed(t, model, "d")
	undoing, undo := pressed(t, completing, "u")
	answered := drain(t, drain(t, undoing, undo), done)

	// Assert: only the done was sent
	requireTaskWrites(t, repo, "task done "+trackedTaskUUID)

	// Act: undo once the done has answered
	typing(t, answered, "u")

	// Assert: the undo goes now
	requireTaskWrites(t, repo, "task done "+trackedTaskUUID, "task undo")
}

func TestTheTasksPaneOffersNoVerbWhileAWriteIsInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	model := typing(t, repo.live(t, 120, 40), tasksPane, downAction)

	// Act
	completing, _ := pressed(t, model, "d")

	// Assert
	view := completing.View().Content
	for _, verb := range []string{offersStart, offersDone, "a add", "A annotate", "e modify", "u undo"} {
		if footer := footerLine(view); strings.Contains(footer, verb) {
			t.Errorf("the footer offers %q while the done is in flight: %q", verb, footer)
		}
	}

	requireScreen(t, view, "sending…")
}

func TestTrackRightAfterTheAddAnswersGoesToTheNewTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	opened := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	sending, send := pressed(t, opened, keyEnter)
	added, _ := finish(t, sending, send) // the add has answered; its annotation has not

	// Act
	view := typing(t, added, "T").View().Content

	// Assert
	requireTaskWrites(t, repo, "task add "+untrackedLine)
	requireScreen(t, view, untrackedIssue+" is tracked by task 5f1c2b3a")
	refuseScreen(t, view, "Track "+untrackedIssue)
}

func TestTrackRightAfterTheAddAnswersSaysTheTaskWasJustAdded(t *testing.T) {
	t.Parallel()

	// Arrange
	// A context is active, but the new task is unlisted because it was just
	// added, not because the context hides it.
	repo := withAnUntrackedIssue()
	repo.tasks.context = "office"
	added := addedButNotAnnotated(t, repo)

	// Act
	view := typing(t, added, "T").View().Content

	// Assert
	requireScreen(t, view, untrackedIssue+" is tracked by task 5f1c2b3a, just added; r in the Tasks pane reads it")
	refuseScreen(t, view, "outside context")
}

func TestAnOfferNamesAJustAddedTaskByItsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// No task is started, so PROJ-500's branch offers to start the task just
	// added for it.
	repo := withAnUntrackedIssue()
	repo.moves = startTransitions()
	stopTheStartedTask(repo)
	added := addedButNotAnnotated(t, repo)

	// Act
	view := typing(t, added, "b", keyEnter, keyEsc).View().Content

	// Assert
	requireScreen(t, view, startLook, "Start task 5f1c2b3a '"+untrackedIssue+": Rotate the keys' in Taskwarrior?")
}

func TestAReadBegunBeforeATrackAnswersKeepsTheNewTask(t *testing.T) {
	t.Parallel()

	// Arrange
	// The tasks are being read again as the track is sent; the read, begun
	// before the add answered, lands after it without the new task.
	repo := withAnUntrackedIssue()
	reading, read := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane), "r")
	opened := typing(t, reading, onTheUntrackedIssue("T")...)
	sending, send := pressed(t, opened, keyEnter)
	added, _ := finish(t, sending, send)
	landed := drain(t, added, read)

	// Act
	view := typing(t, landed, "T").View().Content

	// Assert
	requireScreen(t, view, untrackedIssue+" is tracked by task 5f1c2b3a, just added")
	refuseScreen(t, view, "Track "+untrackedIssue)
}

func TestAReadBegunAfterATrackAnswersReplacesTheNewTask(t *testing.T) {
	t.Parallel()

	// Arrange
	// The task the track added is gone — deleted in a terminal — by the time
	// the read that follows its annotation lands.
	repo := withAnUntrackedIssue()
	opened := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	tracked := typing(t, opened, keyEnter)

	// Act
	view := typing(t, tracked, "T").View().Content

	// Assert
	requireScreen(t, view, "Track "+untrackedIssue)
	refuseScreen(t, view, "is tracked by task 5f1c2b3a")
}

func TestAReadHoldingTheTaskATrackJustAddedLinksItOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// Taskwarrior committed the add before the read begun ahead of its answer
	// ran, so that read holds the new task the stub stands for.
	repo := withAnUntrackedIssue()
	reading, read := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane), "r")
	opened := typing(t, reading, onTheUntrackedIssue("T")...)
	sending, send := pressed(t, opened, keyEnter)
	added, _ := finish(t, sending, send)
	words := untrackedIssue + ": Rotate the keys"
	repo.tasks.linked = append(repo.tasks.linked, taskwarrior.Task{
		UUID: addedTaskUUID, ID: 21, IssueKey: untrackedIssue, Description: words, Status: taskwarrior.Pending,
	})

	// Act
	view := drain(t, added, read).View().Content

	// Assert
	if count := strings.Count(view, words); count != 1 {
		t.Errorf("the issue's Tasks block drew the new task %d times, want once:\n%s", count, view)
	}
}

// addedButNotAnnotated is the world's interface once T has tracked the
// untracked issue and the add has answered, the annotation that follows it
// not yet sent.
func addedButNotAnnotated(t *testing.T, repo *world) tui.Model {
	t.Helper()

	opened := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	sending, send := pressed(t, opened, keyEnter)
	added, _ := finish(t, sending, send)

	return added
}

func TestTrackWaitsForATaskWriteInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withAnUntrackedIssue()
	model := typing(t, repo.live(t, 200, 40), tasksPane, downAction)
	completing, done := pressed(t, model, "d")
	onTheIssue := typing(t, completing, append([]string{"1"}, selectTheUntrackedIssue()...)...)

	// Act
	view := drain(t, typing(t, onTheIssue, "T"), done).View().Content

	// Assert
	refuseScreen(t, view, "Track "+untrackedIssue)
	requireTaskWrites(t, repo, "task done "+trackedTaskUUID)
}

func TestAnOfferWaitsForATaskWriteInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasksToStart()
	model := typing(t, repo.live(t, 200, 40), tasksPane)
	completing, done := pressed(t, model, "d")
	offered := typing(t, completing, append([]string{"1"}, append(branchTheSecondIssue(), keyEsc)...)...)

	// Act: go ahead while the done is on its way
	refused := typing(t, offered, keyEnter)

	// Assert: the offer stays open saying why, and nothing is sent
	requireScreen(t, refused.View().Content, switchLook, "still being sent")
	requireTaskWrites(t, repo)

	// Act: the done answers, then go ahead again
	typing(t, drain(t, refused, done), keyEnter)

	// Assert: the offer's writes follow the done's
	requireTaskWrites(t, repo,
		"task done "+activeTaskUUID, "task stop "+activeTaskUUID, "task start "+trackedTaskUUID)
}

func TestATrackLineWaitsForATaskWriteInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// No task is started, so PROJ-500's branch offers the line that tracks and
	// starts it straight away.
	repo := withAnUntrackedIssue()
	repo.moves = startTransitions()
	stopTheStartedTask(repo)
	completing, done := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane), "d")
	offered := typing(t, completing, onTheUntrackedIssue("b", keyEnter, keyEsc)...)

	// Act: send the line while the done is on its way
	refused := typing(t, offered, keyEnter)

	// Assert: the line stays open saying why
	requireScreen(t, refused.View().Content, trackAndStart+untrackedIssue, "still being sent")

	// Act: the done answers
	drain(t, refused, done)

	// Assert: only the done was sent
	requireTaskWrites(t, repo, "task done "+activeTaskUUID)
}

func TestAnUndoWaitsForTheAnnotationThatFollowsATrack(t *testing.T) {
	t.Parallel()

	// Arrange
	// The add has answered; the annotation with the issue's page has not.
	repo := withAnUntrackedIssue()
	repo.issues[2].Priority = untrackedPriority
	opened := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "T")...)
	sending, send := pressed(t, opened, keyEnter)
	added, annotate := finish(t, sending, send)

	// Act: undo from the Tasks pane before the annotation answers
	undoing := typing(t, added, tasksPane, "u")

	// Assert: no undo is sent
	requireTaskWrites(t, repo, "task add "+untrackedLine)

	// Act: the annotation answers, then undo again
	typing(t, drain(t, undoing, annotate), "u")

	// Assert: the undo follows the annotation
	requireTaskWrites(t, repo,
		"task add "+untrackedLine, "task annotate "+addedTaskUUID+" "+untrackedPage, "task undo")
}

func TestATasksReadLandingMidWriteKeepsTheWriteInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 3's done is on its way when the tasks are read again.
	repo := withTasks()
	completing, _ := pressed(t, typing(t, repo.live(t, 120, 40), tasksPane, downAction), "d")
	reloaded := typing(t, completing, "r")

	// Act
	typing(t, reloaded, "u")

	// Assert
	requireTaskWrites(t, repo)
}

func TestAStopThenTrackWaitsForATaskWriteInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is started, so PROJ-500's branch offers to stop it, then track the
	// issue; task 3's done is on its way.
	repo := withAnUntrackedIssue()
	repo.moves = startTransitions()
	completing, _ := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane, downAction), "d")
	offered := typing(t, completing, onTheUntrackedIssue("b", keyEnter, keyEsc)...)

	// Act
	refused := typing(t, offered, keyEnter)

	// Assert
	requireScreen(t, refused.View().Content, switchLook, "still being sent")
	refuseScreen(t, refused.View().Content, trackAndStart)
	requireTaskWrites(t, repo)
}

// onTheUntrackedIssue is the keys that go back to the Issues pane and onto the
// untracked issue, then keys.
func onTheUntrackedIssue(keys ...string) []string {
	return slices.Concat([]string{"1"}, selectTheUntrackedIssue(), keys)
}
