// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// secondPullURL is the pull request for other work: another branch, for the
// world's second issue.
const secondPullURL = "https://github.com/example/repo/pull/43"

// droppedPost is what the interface says when it gives up on a post that was
// waiting for the world's first pull request.
const droppedPost = "✗ dropped the Slack post waiting for #42, which is no longer this branch's pull request"

// anotherPull makes the world's pull request a different one, whose CI has
// passed.
func anotherPull(w *world) {
	w.pull = forge.PullRequest{Number: 43, URL: secondPullURL, Title: "feat(jira): add retries", Draft: false}
	w.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1, Failed: 0}}
}

// switchToOtherWork moves the world to another branch with a pull request of
// its own, as `git switch` in another terminal would.
func switchToOtherWork(w *world) {
	w.branch.Name = "feat/" + secondIssue + "-add-retries"
	w.branch.Upstream = "origin/" + w.branch.Name

	anotherPull(w)
}

// switchBack returns the world to the branch and pull request it starts on.
func switchBack(w *world) {
	first := newWorld()
	w.branch, w.pull = first.branch, first.pull
}

func TestAPostWaitingForCIIsNotSentForAnotherPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	switching := newWorld()
	switching.ci = []forge.CI{{State: forge.CIRunning}}
	model := switching.live(t, 120, 40)

	// Act: leave the post waiting for the first pull request's CI
	waiting := typing(t, model, "5", "p", "w")

	// Assert: it waits
	requireScreen(t, waiting.View().Content, "◐ will post to "+slackChannel+" once CI passes")

	// Act: switch to other work, whose CI has passed
	switchToOtherWork(switching)

	switched := typing(t, waiting, "2", "r")

	// Assert: what was written for the first is not posted for the second, and
	// the interface says it gave up on it
	if calls := switching.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request it was not written for", calls)
	}

	requireScreen(t, switched.View().Content, droppedPost)

	// Act: look at the Slack pane
	pane := typing(t, switched, "5").View().Content

	// Assert: nothing is waiting any more
	requireScreen(t, pane, "state  ○ nothing posted")
}

func TestAPostWaitingForCIIsDroppedWhenThePullRequestIsReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pull request is closed and another opened from the same branch while
	// a post written for the first one waits.
	replaced := newWorld()
	replaced.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, replaced.live(t, 120, 40), "5", "p", "w")

	anotherPull(replaced)

	// Act
	refreshed := typing(t, waiting, "4", "r")

	// Assert
	if calls := replaced.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request it was not written for", calls)
	}

	requireScreen(t, refreshed.View().Content, droppedPost)
}

func TestEachPullRequestIsAnnouncedOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	busy := newWorld()
	first := typing(t, busy.live(t, 120, 40), "5", "p", keyEnter)

	// Act: switch to other work
	switchToOtherWork(busy)

	second := typing(t, first, "2", "r", "5").View().Content

	// Assert: its pull request has not been announced, and can be
	requireScreen(t, second, "state  ○ nothing posted", "○ Slack")
	requireScreen(t, footerLine(second), "p post to slack")

	// Act: announce it
	both := typing(t, first, "2", "r", "5", "p", keyEnter)

	// Assert: the second post is about the second pull request
	calls := busy.asked("post ")
	if len(calls) != 2 || !strings.Contains(calls[1], secondPullURL) {
		t.Fatalf("post calls = %q, want a second one linking %s", calls, secondPullURL)
	}

	// Act: go back to the first
	switchBack(busy)

	back := typing(t, both, "2", "r", "5").View().Content

	// Assert: it is still announced, and is not offered again
	requireScreen(t, back, "state  ● posted")
	refuseScreen(t, footerLine(back), "p post")
}

func TestAPostWaitingForCIDoesNotOutliveABranchSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	away := newWorld()
	away.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, away.live(t, 120, 40), "5", "p", "w")

	// Act: switch to a branch with no pull request
	away.branch.Name, away.pullFound = "feat/"+secondIssue+"-add-retries", false

	elsewhere := typing(t, waiting, "2", "r")

	// Assert: the interface gives up on the post there and then, and says so
	requireScreen(t, elsewhere.View().Content, droppedPost)

	// Act: come back once CI has passed
	switchBack(away)

	away.pullFound, away.ci = true, []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1, Failed: 0}}

	typing(t, elsewhere, "2", "r")

	// Assert: it does not post on its own long after it was asked to
	if calls := away.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q on coming back to the branch", calls)
	}
}
