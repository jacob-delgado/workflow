// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CI polling backs off once a check fails: a failed check would only fail again
// at the same rate, so the interface stops asking until the next refresh rather
// than hammering the forge for as long as it runs.

import (
	"errors"
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
