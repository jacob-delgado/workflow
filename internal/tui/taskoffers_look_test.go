// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

func TestConfirmingTheStatusAfterABranchStillOffersToStartTheTask(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasksToStart()
	stopTheStartedTask(repo)
	branched := typing(t, repo.live(t, 200, 40), branchTheSecondIssue()...)

	// Act
	// The status picker is on its in-progress move.
	view := typing(t, branched, keyEnter).View().Content

	// Assert
	// PROJ-388 moved, and the offer that waited behind the picker opens.
	if len(repo.asked("transition "+secondIssue+" 11")) == 0 {
		t.Fatalf("%s was never moved to Doing", secondIssue)
	}

	requireScreen(t, view, startLook, "Start task 3 '"+secondIssue+": Add retries'")
}

func TestApplyingTheReviewStatusStillOffersToNoteThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request is linked on PROJ-412, and the status picker is on its
	// review status.
	repo := withTasks()
	repo.pullFound = false
	repo.cfg.Jira.ReviewStatus = statusInReview
	repo.moves = reviewTransitions()
	picking := typing(t, repo.live(t, 200, 40), "4", "n", keyEnter, keyEnter)

	// Act
	view := typing(t, picking, keyEnter).View().Content

	// Assert
	// PROJ-412 moved, and the offer that waited behind the linker opens.
	if len(repo.asked("transition "+issueKey+" 31")) == 0 {
		t.Fatalf("%s was never moved to %s", issueKey, statusInReview)
	}

	requireScreen(t, view, noteLook, "Annotate task 12 with #42 and its URL?")
}

func TestMovingToDoneWithNothingToCompleteStillOffersToTrackAndStart(t *testing.T) {
	t.Parallel()

	// Arrange
	// No task tracks PROJ-500, so moving it to Done offers nothing of its own.
	repo := withAnUntrackedIssue()
	stopTheStartedTask(repo)
	repo.moves = startTransitions()
	repo.moves = append(repo.moves, transition("31", "Done", "Done", "done"))
	branched := typing(t, repo.live(t, 200, 40), append(selectTheUntrackedIssue(), "b", keyEnter)...)

	// Act
	view := typing(t, branched, downAction, keyEnter).View().Content

	// Assert
	// PROJ-500 moved, and the offer that waited behind the picker opens.
	if len(repo.asked("transition "+untrackedIssue+" 31")) == 0 {
		t.Fatalf("%s was never moved to Done", untrackedIssue)
	}

	requireScreen(t, view, trackAndStart+untrackedIssue)
}

func TestAnOffersChangeReadsTheTasksAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withTasks()
	switching := loopMoments()["switching tasks"]
	switching.arrange(repo)
	offered := typing(t, repo.live(t, 200, 40), switching.keys...)
	before := len(repo.asked("tasks"))

	// Act
	typing(t, offered, keyEnter)

	// Assert
	if after := len(repo.asked("tasks")); after <= before {
		t.Errorf("the tasks were read %d times before the offer's change and %d after: never again", before, after)
	}
}

func TestATaskChangeFromTheTasksPaneLeavesAnotherLookInFlight(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		writeErr error
		told     string
	}{
		"made":    {writeErr: nil, told: "stopped 12"},
		"refused": {writeErr: hookRefused(), told: hookRefusal},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Task 12 is being stopped from the Tasks pane as the checks are re-run.
			repo := withTasks()
			repo.ci = failedThenRunning()
			repo.tasks.writeErr = tt.writeErr
			stopping, stop := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane), "s")
			rerunning, _ := pressed(t, typing(t, stopping, "4", "R"), keyEnter)

			// Act
			view := drain(t, rerunning, stop).View().Content

			// Assert
			// The task's answer is told, and the re-run's look still waits on its own.
			requireScreen(t, view, tt.told, "Re-run checks", "re-running…")
		})
	}
}

func TestAnOfferNotYetConfirmedOutlivesAnotherTaskChange(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12 is being stopped from the Tasks pane as the pull request merges.
	repo := withTasks()
	readyToMerge(repo)
	stopping, stop := pressed(t, typing(t, repo.live(t, 200, 40), tasksPane), "s")
	offered := typing(t, stopping, "4", "M", keyEnter)

	// Act
	view := drain(t, offered, stop).View().Content

	// Assert
	requireScreen(t, view, "stopped 12", completeLook)
}
