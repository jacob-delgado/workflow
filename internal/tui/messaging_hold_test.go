// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// An announcement armed to go once CI passes is one hold, read on a beat of
// its own and bound to the branch and pull request it was armed for, never
// to the checkout or the pull request on screen. Quitting asks before losing
// it from every way out; a post that may have gone out is never posted again
// unasked, while one the service asked to wait is tried again.

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// Why a post failed: the service answered a status it does not document, so
// the post may have gone out; or it asked the post to wait.
var (
	errServiceAnswered500 = fmt.Errorf("%w: 500", messaging.ErrUnexpectedStatus)
	errAskedToWait        = fmt.Errorf("posting: %w", httpx.ErrRateLimited)
)

// otherWorkURL is the pull request opened from the other work's branch.
const otherWorkURL = "https://github.com/example/repo/pull/44"

// postsNow lets the world's posts through from the next one on.
func postsNow(repo *world) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.postErr = nil
}

// keptOn is the world switched to other work, as switchToOtherWork does,
// with the branch it left keeping its pull request, which the forge still
// finds by that branch.
func keptOn(repo *world) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	repo.pullsOf = map[string]forge.PullRequest{repo.branch.Name: repo.pull}
	switchToOtherWork(repo)
}

func TestQFromTheHelpAsksBeforeLosingAWaitingAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	_, waiting := waitingOn(t, forge.CIRunning)
	help := typing(t, waiting, "?")

	// Act
	asked, cmd := pressed(t, help, "q")

	// Assert
	if messageType(cmd) == messageType(tea.Quit) {
		t.Error("q in the help quit at once, losing the announcement waiting for CI")
	}

	requireScreen(t, asked.View().Content, "An announcement is waiting for CI and will be lost.")
}

func TestAnArmedPostThatMayHaveGoneOutIsNotPostedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	repo.postErr = errServiceAnswered500
	armed := opens(t, repo)
	passesNow(repo)

	// Act: CI passes, and the post's outcome is unknown
	uncertain := typing(t, armed, "r")

	// Assert: it says it may have gone out
	requireScreen(t, uncertain.View().Content, "may have gone out")

	// Act: read again
	reread := typing(t, uncertain, "r").View().Content

	// Assert: it was posted once, and is offered only to workflow announce
	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("post calls = %q, want one: a post that may have gone out is not posted again unasked", posts)
	}

	requireScreen(t, reread, "was not confirmed; workflow announce posts it again")
}

func TestAnArmedPostTheServiceAskedToWaitIsTriedAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	repo.postErr = errAskedToWait
	armed := opens(t, repo)
	passesNow(repo)

	// Act: CI passes, and the service asks the post to wait
	asked := typing(t, armed, "r")

	// Assert: it waits to try again
	requireScreen(t, asked.View().Content, "announces when CI passes", "trying again")

	// Act: the service takes posts again, and the hold reads again
	postsNow(repo)

	posted := typing(t, asked, "r").View().Content

	// Assert: the second try went out
	if posts := repo.asked("post "); len(posts) != 2 {
		t.Errorf("post calls = %q, want the one asked to wait and the one tried again", posts)
	}

	requireScreen(t, posted, "● announced")
}

func TestRemovingTheLocalDataLetsTheSessionAnnounceAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	announced := typing(t, repo.live(t, 140, 40), "5", "p", keyEnter)

	// Act
	removed := typing(t, announced, reposKey, localDataKey, "c", keyEnter, "5").View().Content

	// Assert
	requireScreen(t, footerLine(removed), "p announce to Slack")
}

func TestWhenCIPassesReadsCIAtItsIntervalWhileANotificationWaitsLonger(t *testing.T) {
	t.Parallel()

	// Arrange
	// Being told CI finished polls on a slower beat with no interval set; a
	// post waiting for CI must not wait on it.
	repo := newWorld()
	repo.cfg.UI.Notify = true
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}
	timer := &heldTimer{}
	preview := typing(t, timed(t, repo, timer.after), "5", "p")
	before := len(timer.waits())

	// Act
	typing(t, preview, "w")

	// Assert
	if waits := timer.waits()[before:]; !slices.Contains(waits, config.DefaultCIInterval) {
		t.Errorf("waits asked for once w was pressed = %v, want one of the CI interval, %v", waits,
			config.DefaultCIInterval)
	}
}

func TestAnAnnouncementArmedAsItsPullRequestOpenedWaitsForTheForgeToListIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := openingWorld()
	opened := typing(t, repo.live(t, 160, 40), "4", "n", keyEnter, keyEnter, keyEsc, keyEsc, "5")

	// Act: read again, the forge still listing no pull request for the branch
	unlisted := typing(t, opened, "r")

	// Assert: it waits for the forge to list it
	requireScreen(t, unlisted.View().Content, "waiting for the forge to list #42")

	// Act: the forge lists it, CI passes, and it is read again
	repo.mu.Lock()
	repo.pullFound = true
	repo.mu.Unlock()
	passesNow(repo)
	typing(t, unlisted, "r")

	// Assert: it is announced, once
	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("post calls = %q, want the pull request announced once it was listed", posts)
	}
}

func TestAPostWaitingForCIGoesForItsOwnPullRequestAfterABranchSwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits for #42's CI; the checkout switches to other work, #43,
	// whose CI has passed, and #42's CI then passes too.
	repo, waiting := waitingOn(t, forge.CIRunning)
	keptOn(repo)

	switched := typing(t, waiting, "2", "r", "5")

	// Act
	typing(t, switched, "r")

	// Assert
	posts := repo.asked("post ")
	if len(posts) != 1 || !strings.Contains(posts[0], pullURL) || strings.Contains(posts[0], secondPullURL) {
		t.Errorf("post calls = %q, want #42's announcement, the one written, posted once", posts)
	}
}

func TestAPreviewOpenedOnAFailedCheckPostsOnceARunHasPassed(t *testing.T) {
	t.Parallel()

	// Arrange
	// A check had failed when the preview opened; by the time it posts, a
	// run has passed.
	repo := newWorld()
	repo.ci = []forge.CI{failedCI(), {State: forge.CIPassed, Total: 1, Done: 1}}
	preview := typing(t, repo.live(t, 140, 40), "5", "p")

	// Act
	typing(t, preview, keyEnter)

	// Assert
	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("post calls = %q, want it posted: the gate's own read found the run passed", posts)
	}
}

func TestWhenCIPassesOverAnotherPullRequestsWaitingPostReplacesItAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	keptOn(repo)
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1}}
	switched := typing(t, waiting, "2", "r", "5")

	// Act
	replaced := typing(t, switched, "p", "w").View().Content

	// Assert
	requireScreen(t, replaced, "replaced the announcement waiting for #42")
}

func TestOpeningAPullRequestNeverReplacesAnotherWaitingAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// #42's announcement was armed as it opened; the checkout moves to other
	// work, with no pull request yet.
	repo := openingWorld()
	armed := opens(t, repo)

	repo.mu.Lock()
	repo.pullsOf = map[string]forge.PullRequest{repo.branch.Name: repo.pull}
	repo.branch.Name = "feat/" + secondIssue + "-add-retries"
	repo.branch.Upstream = "origin/" + repo.branch.Name
	repo.pull = forge.PullRequest{Number: 44, URL: otherWorkURL, Title: "feat(jira): add retries", HeadCommit: headCommit}
	repo.pullFound = false
	repo.mu.Unlock()

	moved := typing(t, armed, "2", "r")

	// Act
	opened := typing(t, moved, "4", "n", keyEnter, keyEnter).View().Content

	// Assert
	requireScreen(t, opened, "#42 is still waiting, so #44 was not armed")
}

func TestAnAnnounceNowInsteadThatFailsKeepsThePostWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	repo, waiting := waitingOn(t, forge.CIRunning)
	repo.postErr = errNotInChannel

	// Act: announce now instead, which the service refuses
	refused := typing(t, waiting, "p", keyEnter, keyEsc)

	// Assert: the post still waits
	requireScreen(t, footerLine(refused.View().Content), "x stop waiting")

	// Act: the service takes posts again, CI passes, and the hold reads again
	postsNow(repo)
	passesNow(repo)

	posted := typing(t, refused, "r").View().Content

	// Assert: the post that waited went out
	if posts := repo.asked("post "); len(posts) != 2 {
		t.Errorf("post calls = %q, want the refused one and the one that waited", posts)
	}

	requireScreen(t, posted, "● announced")
}
