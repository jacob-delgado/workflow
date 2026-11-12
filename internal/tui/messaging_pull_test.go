// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// secondPullURL is the pull request for other work: another branch, for the
// world's second issue.
const secondPullURL = "https://github.com/example/repo/pull/43"

// droppedPost is what the interface says when it gives up on an announcement
// that was waiting for the world's first pull request.
const droppedPost = "✗ dropped the announcement to Slack waiting for #42: #42 is no longer its branch's pull request"

// anotherPull makes the world's pull request a different one, whose CI has
// passed.
func anotherPull(w *world) {
	w.pull = forge.PullRequest{
		Number: 43, URL: secondPullURL, Title: "feat(jira): add retries", Draft: false, HeadCommit: "fed9876",
	}
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
	requireScreen(t, waiting.View().Content, "◐ will announce to "+slackChannel+" once CI passes")

	// Act: switch to other work, whose CI has passed, and look at the Slack
	// pane
	keptOn(switching)

	pane := typing(t, waiting, "2", "r", "5").View().Content

	// Assert: what was written for the first is not posted for the second; it
	// waits on for the first
	if calls := switching.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request it was not written for", calls)
	}

	requireScreen(t, pane, "state  ◐ announces #42 when CI passes")
	requireScreen(t, footerLine(pane), "x stop waiting", "p announce to Slack")
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
	refreshed := typing(t, waiting, "r")

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
	requireScreen(t, second, "state  ○ nothing announced", "○ Slack")
	requireScreen(t, footerLine(second), "p announce to Slack")

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
	requireScreen(t, back, "state  ● announced")
	refuseScreen(t, footerLine(back), "p announce")
}

func TestAPullRequestClosedWithoutMergingIsNothingToAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	// The store remembers pull request 42 announced as ready, before it was
	// closed without merging: the loop follows no pull request now, so the
	// pane reads it as the top row does, with nothing announced or to announce.
	closed := newWorld()
	closed.pull.State = forge.StateClosed
	closed.storedAnnounces = []loop.Announced{{Pull: 42, Moment: messaging.MomentReady}}

	// Act
	view := typing(t, closed.live(t, 120, 40), "5").View().Content

	// Assert
	requireScreen(t, view, "Open a pull request first (4 Review)", "○ Slack")
	refuseScreen(t, view, "● announced")
	refuseScreen(t, footerLine(view), "p announce")
}

func TestAPostWaitingForCIOutlivesABranchSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	// The post is bound to the branch it was armed on, which keeps its pull
	// request whatever the checkout; x stops it.
	away := newWorld()
	away.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, away.live(t, 120, 40), "5", "p", "w")

	// Act: switch to a branch with no pull request
	away.pullsOf = map[string]forge.PullRequest{away.branch.Name: away.pull}
	away.branch.Name, away.pullFound = "feat/"+secondIssue+"-add-retries", false

	elsewhere := typing(t, waiting, "2", "r", "5")

	// Assert: it waits on, saying for which pull request
	requireScreen(t, elsewhere.View().Content, "◐ announces #42 when CI passes")
	refuseScreen(t, elsewhere.View().Content, "dropped the announcement")

	// Act: its CI passes, and it is read again
	away.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1, Failed: 0}}

	typing(t, elsewhere, "r")

	// Assert: it is posted, once
	if calls := away.asked("post "); len(calls) != 1 {
		t.Errorf("post calls = %q, want the waiting post sent once its CI passed", calls)
	}
}

func TestAnAnnouncementDroppedForAnotherPullRequestNamesTheServiceItWasFor(t *testing.T) {
	t.Parallel()

	// Arrange
	switching := newWorld()
	switching.cfg.Messaging = teamsMessaging()
	switching.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, switching.live(t, 200, 40), "5", "p", "w")

	anotherPull(switching)

	// Act
	replaced := typing(t, waiting, "r")

	// Assert
	requireScreen(t, replaced.View().Content, "✗ dropped the announcement to Teams waiting for #42")
}
