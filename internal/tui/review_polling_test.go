// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CI polling backs off once a check fails: a failed check would only fail again
// at the same rate, so the interface stops asking until the next refresh rather
// than hammering the forge for as long as it runs. A failed find for the pull
// request is not a failed check, and the poll carries on through it.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errForgeDown is a CI check that failed, for the back-off path.
var errForgeDown = errors.New("the forge is down")

func TestPollingStopsAndShowsTheErrorAfterAFailedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI is running, but every check of it fails. The interval is short enough that
	// an unbounded poll would fire many times within the harness's horizon.
	failing := newWorld()
	failing.ci = []forge.CI{{State: forge.CIRunning, Total: 1, Done: 0}}
	failing.ciErr = errForgeDown
	failing.ciInterval = pollTick

	// Act
	settled := failing.live(t, 120, 40)

	// Assert
	// Polling stops after the first failed check, and the pane says why.
	if checks := failing.asked("ci"); len(checks) != 1 {
		t.Errorf("CI was checked %d times, want once before backing off: %v", len(checks), checks)
	}

	requireScreen(t, settled.View().Content, errForgeDown.Error())
}

func TestComingBackToABranchBeforeItsPollKeepsOnePollingChain(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs on the branch's pull request and a poll is scheduled. The developer
	// switches to a branch with no pull request and back before it is due, so the
	// first visit's poll fires after the second visit has scheduled its own.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	timer := &heldTimer{}
	screen := timed(t, repo, timer.after)

	repo.branch.Name, repo.pullFound = "feat/"+secondIssue+"-add-retries", false
	away := typing(t, screen, "2", "r")

	switchBack(repo)

	repo.pullFound = true
	back := typing(t, away, "r")
	checked := len(repo.asked("ci "))

	// Act
	timer.release(t, back)

	// Assert
	if asked := len(repo.asked("ci ")) - checked; asked != 1 {
		t.Errorf("the polls due asked about CI %d times, want once: the first visit's poll must end its chain", asked)
	}

	if chains := timer.waiting(); chains != 1 {
		t.Errorf("%d polls are scheduled after them, want one: a chain for the pull request, not two", chains)
	}
}

func TestRefreshingTheSamePullRequestKeepsItsPollingChain(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh that finds the same pull request keeps the poll already scheduled,
	// rather than ending its chain or starting a second beside it.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	timer := &heldTimer{}
	screen := timed(t, repo, timer.after)

	// Act: refresh the review
	refreshed := typing(t, screen, "4", "r")
	checked := len(repo.asked("ci "))

	// Assert: the refresh schedules no poll beside the one already held
	if held := timer.waiting(); held != 1 {
		t.Errorf("%d polls held after the refresh, want 1: it must not end the chain and start another", held)
	}

	// Act: the poll falls due
	timer.release(t, refreshed)

	// Assert: it asks once and schedules the next
	if asked := len(repo.asked("ci ")) - checked; asked != 1 {
		t.Errorf("the poll due asked about CI %d times, want once", asked)
	}

	if chains := timer.waiting(); chains != 1 {
		t.Errorf("%d polls are scheduled after it, want one: the refresh must neither end the chain nor add one", chains)
	}
}

func TestAFailedFindKeepsAQueuedPostWaitingForCI(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits on CI that is running when the interface starts and when w is
	// pressed, and has passed by the poll held for it. Before that poll falls due,
	// a reload of the branch finds no pull request, failing as the forge does when
	// it cannot be reached.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIRunning}, {State: forge.CIPassed, Total: 1, Done: 1}}
	timer := &heldTimer{}
	waiting := typing(t, timed(t, repo, timer.after), "5", "p", "w")

	repo.pullFound, repo.pullErr = false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)
	reloaded := typing(t, waiting, "2", "r")

	// Act
	timer.release(t, reloaded)

	// Assert
	if posts := repo.asked("post "); len(posts) != 1 {
		t.Errorf("posted %d times once CI passed, want once: a failed find must not end the poll the post waits on",
			len(posts))
	}
}

func TestAFailedFindReadsCIAgainForTheNewHead(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI passed on the head the pull request was found at, so it can be merged. A
	// commit then moves the head, and the reload that follows cannot find the pull
	// request.
	repo := mergeable()
	repo.ci = []forge.CI{{State: forge.CIPassed, Total: 1, Done: 1}, {State: forge.CIRunning, Total: 1}}
	model := repo.live(t, 120, 40)
	repo.branch.Head = "def456"
	repo.pullFound, repo.pullErr = false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)

	// Act
	view := typing(t, model, "2", "r", "4").View().Content

	// Assert
	if checks := repo.asked("ci def456"); len(checks) != 1 {
		t.Errorf("CI on the new head was checked %d times, want once: %q", len(checks), repo.asked("ci "))
	}

	refuseScreen(t, footerLine(view), "M merge")
}

func TestAFindThatSucceedsClearsTheFailedFindBeforeIt(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	model := repo.live(t, 120, 40)
	repo.pullFound, repo.pullErr = false, fmt.Errorf("finding the pull request: %w", forge.ErrUnreachable)

	// Act: a reload's find fails
	failed := typing(t, model, "2", "r")

	// Assert: the failure shows
	requireScreen(t, failed.View().Content, "could not reach the forge")

	// Act: the forge answers again, and the next reload's find succeeds
	repo.pullFound, repo.pullErr = true, nil
	recovered := typing(t, failed, "2", "r")

	// Assert: the failure is gone, and the pull request shows in its place
	refuseScreen(t, recovered.View().Content, "could not reach the forge")
	requireScreen(t, recovered.View().Content, "#42 "+pullTitle)
}
