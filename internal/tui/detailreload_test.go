// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The selected issue is read in full when the selection rests on it, and
// again on r or after a change to it. A reload that fails keeps the issue
// shown, beside its failure; a read that failed is tried again once the
// selection comes back to it.

import "testing"

func TestAFailedReloadKeepsTheIssueShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issue is shown in full; then Jira stops showing it.
	failing := newWorld()
	shown := failing.live(t, 120, 40)
	failing.detailErr = errNotVisible

	// Act
	view := typing(t, shown, "r").View().Content

	// Assert
	requireScreen(t, view, "Tokens reach the log.", "Issue Does Not Exist", "press r to try again")
}

func TestComingBackToAnIssueWhoseReadFailedReadsItAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issue's read failed; the selection leaves it and comes back
	// before the next issue is read.
	failing := newWorld()
	failing.detailErr = errNotVisible
	away, rest := pressed(t, failing.live(t, 120, 40), "j")
	failing.detailErr = nil
	back := typing(t, away, "k")

	// Act
	view := drain(t, back, rest).View().Content

	// Assert
	requireScreen(t, view, "Tokens reach the log.")
	refuseScreen(t, view, errNotVisible.Error())
}

func TestAnAnswerThatKeepsTheSelectionDoesNotReadAFailedIssueAgain(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		world func() *world
		keys  []string
	}{
		"a branch read": {world: newWorld, keys: []string{"2", "r"}},
		"a tasks read":  {world: withTasks, keys: []string{"7", "r"}},
	}

	for name, answering := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Jira refuses the selected issue; another pane's read answers
			// and leaves the selection where it was.
			refusing := answering.world()
			refusing.detailErr = errNotVisible
			model := refusing.live(t, 120, 40)
			before := len(refusing.asked("issue " + issueKey))

			// Act
			typing(t, model, answering.keys...)

			// Assert
			if after := len(refusing.asked("issue " + issueKey)); after != before {
				t.Errorf("asked Jira for the issue %d times, want the %d before: only a move or r reads it again", after, before)
			}
		})
	}
}

func TestAMoveWhileAFailedIssueIsReadAgainAsksForItNoMore(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issue's read failed, and r reads it again, slowly; k at the top
	// of the list leaves the selection on it.
	failing := newWorld()
	failing.detailErr = errNotVisible
	retrying, _ := withheldKeys(t, failing.live(t, 120, 40), failing.counting("issue "), "r")
	before := len(failing.asked("issue " + issueKey))

	// Act
	typing(t, retrying, "k")

	// Assert
	if after := len(failing.asked("issue " + issueKey)); after != before {
		t.Errorf("asked Jira for the issue %d more time(s), want none while its read is out", after-before)
	}
}

func TestAMoveInAnEmptyListAsksForNoIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	empty := newWorld()
	empty.issues = nil
	model := empty.live(t, 120, 40)

	// Act
	typing(t, model, "j")

	// Assert
	if asked := empty.asked("issue "); len(asked) != 0 {
		t.Errorf("asked Jira for %q, want no issue with none listed", asked)
	}
}
