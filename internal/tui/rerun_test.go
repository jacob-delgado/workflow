// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// failedThenRunning is a failed CI that, once re-run, reads as running again.
func failedThenRunning() []forge.CI {
	return []forge.CI{
		{State: forge.CIFailed, Total: 3, Done: 3, Failed: 1},
		{State: forge.CIRunning, Total: 3, Done: 0},
	}
}

func TestRerunOnAFailedPullRequestRestartsTheChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := newWorld()
	reviewing.ci = failedThenRunning()

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "CI     ◐ running")

	if calls := reviewing.asked("rerun abc123"); len(calls) != 1 {
		t.Errorf("rerun calls = %q, want one for the head commit", reviewing.asked("rerun"))
	}
}

func TestARerunThatFailsPlainlySaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	// The re-run fails for a reason that is not a missing scope.
	reviewing := newWorld()
	reviewing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	reviewing.rerunErr = forge.ErrRejected

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "re-run failed")
	refuseScreen(t, view, "checks write scope")
}

func TestRerunSaysWhenThereIsNothingToReRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failure has no re-runnable job, so the pane must not claim one started.
	reviewing := newWorld()
	reviewing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	reviewing.nothingToRerun = true

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "nothing to re-run", "CI     ✗ failed")
}

func TestRerunIsNotOfferedWhileChecksPass(t *testing.T) {
	t.Parallel()

	// Arrange
	// newWorld's CI has passed, so there is nothing to re-run.
	reviewing := newWorld()

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	refuseScreen(t, view, "re-run checks")

	if calls := reviewing.asked("rerun"); len(calls) != 0 {
		t.Errorf("re-ran passing checks: %q", calls)
	}
}

func TestRerunIsNotOfferedOnceThePullMerges(t *testing.T) {
	t.Parallel()

	// Arrange
	// The checks failed — a re-run would be offered — until the pull merges,
	// leaving the pane's last-read CI still failed.
	reviewing := newWorld()
	reviewing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	onReview := typing(t, reviewing.live(t, 120, 40), "4")
	reviewing.pull.State = forge.StateMerged

	// Act
	refreshed := typing(t, onReview, "r", "R", keyEnter)

	// Assert
	refuseScreen(t, refreshed.View().Content, "re-run checks")

	if calls := reviewing.asked("rerun"); len(calls) != 0 {
		t.Errorf("re-ran an already-merged pull request: %q", calls)
	}
}

func TestARefusedRerunNamesTheMissingScope(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := newWorld()
	reviewing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	reviewing.rerunErr = forge.ErrRefused

	// Act
	view := typing(t, reviewing.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "the token needs a checks write scope")
}

func TestADryRunReRunsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would re-run the failed checks")
	refuseScreen(t, view, "Re-run checks")

	if calls := dry.asked("rerun"); len(calls) != 0 {
		t.Errorf("a dry run re-ran the checks: %q", calls)
	}
}

func TestRerunAsksBeforeTheRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := newWorld()
	reviewing.ci = failedThenRunning()
	onReview := typing(t, reviewing.live(t, 120, 40), "4")

	// Act: press R on the failed pull request
	look := typing(t, onReview, "R")

	// Assert: the look names the pull request, and the forge is asked nothing yet
	requireScreen(t, look.View().Content, "Re-run checks", "#42 "+pullTitle, "enter re-run", "esc close")

	if calls := reviewing.asked("rerun"); len(calls) != 0 {
		t.Fatalf("re-ran before the look was confirmed: %q", calls)
	}

	// Act: back out
	backedOut := typing(t, look, keyEsc)

	// Assert: the look closes, and esc asks nothing
	refuseScreen(t, backedOut.View().Content, "Re-run checks")

	if calls := reviewing.asked("rerun"); len(calls) != 0 {
		t.Fatalf("esc re-ran the checks: %q", calls)
	}

	// Act: open it again and go ahead
	confirmed := typing(t, backedOut, "R", keyEnter)

	// Assert: enter asks for the re-run once, and the pane watches CI run again
	if calls := reviewing.asked("rerun abc123"); len(calls) != 1 {
		t.Errorf("rerun calls = %q, want one for the head commit", reviewing.asked("rerun"))
	}

	requireScreen(t, confirmed.View().Content, "CI     ◐ running")
}

func TestRefusedRerunStaysInItsLastLook(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := newWorld()
	reviewing.ci = []forge.CI{{State: forge.CIFailed, Total: 1, Done: 1, Failed: 1}}
	reviewing.rerunErr = forge.ErrRefused
	look := typing(t, reviewing.live(t, 120, 40), "4", "R")

	// Act: go ahead, and the forge refuses
	refused := typing(t, look, keyEnter)

	// Assert: the look is still open, the refusal pinned in it, ready to try again
	requireScreen(t, refused.View().Content, "Re-run checks", "✗ the forge refused the request", "enter re-run")

	// Act: close it
	closed := typing(t, refused, keyEsc)

	// Assert: the look is gone, and the refused re-run was asked for once
	refuseScreen(t, closed.View().Content, "Re-run checks", "✗ the forge refused the request")

	if calls := reviewing.asked("rerun"); len(calls) != 1 {
		t.Errorf("rerun calls = %q, want the one refused", calls)
	}
}
