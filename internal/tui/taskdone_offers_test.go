// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// A task just marked done is still listed as the read before the done had
// it until a read begun after the done answers, so the moments of the loop
// that offer a change of a task must leave it alone meanwhile, as its keys
// on the Tasks pane do.

// markedDoneUnread marks done the task the rows reach from the top of the
// Tasks pane, and leaves the read after the done unanswered.
func markedDoneUnread(t *testing.T, repo *world, rows ...string) tui.Model {
	t.Helper()

	opened := typing(t, repo.live(t, 200, 40), append(append([]string{tasksPane}, rows...), "d")...)
	sending, send := pressed(t, opened, keyEnter)
	answered, _ := finish(t, sending, send)

	return answered
}

func TestNoMomentOffersAChangeOfATaskJustMarkedDone(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		moment moment
		// rows reach the task marked done from the top of the Tasks pane:
		// none for task 12, which tracks PROJ-412, and one for task 3, which
		// tracks PROJ-388.
		rows []string
	}{
		"merging, once task 12 is marked done": {
			moment: moment{arrange: readyToMerge, keys: []string{"4", "M", keyEnter}, reached: "merge 42"},
		},
		"moving the issue to done, once task 12 is marked done": {
			moment: moment{
				arrange: func(repo *world) { repo.moves = workflowMoves() },
				keys:    []string{"1", "t", downAction, keyEnter}, reached: "transition " + issueKey + " 31",
			},
		},
		"opening a pull request, once task 12 is marked done": {
			moment: moment{
				arrange: func(repo *world) { repo.pullFound = false },
				keys:    []string{"4", "n", keyEnter, keyEsc}, reached: "open ",
			},
		},
		"switching to PROJ-388's branch, once task 3 is marked done": {
			moment: loopMoments()["switching branches"],
			rows:   []string{downAction},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := withTasks()
			tt.moment.arrange(repo)
			model := markedDoneUnread(t, repo, tt.rows...)

			// Act
			view := typing(t, model, tt.moment.keys...).View().Content

			// Assert
			if len(repo.asked(tt.moment.reached)) == 0 {
				t.Fatalf("%q was never asked for: the moment was not reached", tt.moment.reached)
			}

			refuseScreen(t, view, offerTitles()...)
		})
	}
}

func TestATaskJustMarkedDoneIsNotStoppedBeforeAnotherStarts(t *testing.T) {
	t.Parallel()

	// Arrange
	// Task 12, the one started, is marked done: Taskwarrior stops a task it
	// completes, so starting task 3 stops nothing.
	repo := withTasks()
	switching := loopMoments()["switching branches"]
	switching.arrange(repo)
	model := markedDoneUnread(t, repo)

	// Act
	view := typing(t, model, switching.keys...).View().Content

	// Assert
	requireScreen(t, view, startLook, "Start task 3 '"+secondIssue+": Add retries' in Taskwarrior?")
	refuseScreen(t, view, switchLook, stopTwelve)
}
