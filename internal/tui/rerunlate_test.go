// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A re-run is started on the forge a moment after the forge says it has
// been, so the first read after it can still list the failure re-run.

import (
	"context"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// rerunCheck names the check a re-run re-runs.
const rerunCheck = "unit-race"

// failedRun is a failed CI of one check, a run whose log the forge serves,
// with the run's id.
func failedRun(id string) forge.CI {
	return forge.CI{
		State: forge.CIFailed, Total: 1, Done: 1, Failed: 1,
		Checks: []forge.Check{{ID: id, Name: rerunCheck, State: forge.CIFailed, LogAvailable: true}},
	}
}

func TestAReadStillListingTheFailureReRunWaitsForTheReRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first read after the re-run still lists the run that failed; the
	// re-run then shows running, under an id of its own, and passes.
	repo := newWorld()
	repo.cfg.UI.Notify = true
	repo.ciInterval = pollTick
	repo.ci = []forge.CI{
		failedRun("11"),
		failedRun("11"),
		{State: forge.CIRunning, Total: 1, Checks: []forge.Check{{ID: "12", Name: rerunCheck, State: forge.CIRunning}}},
		{
			State: forge.CIPassed, Total: 1, Done: 1,
			Checks: []forge.Check{{ID: "12", Name: rerunCheck, State: forge.CIPassed}},
		},
	}

	// Act
	view := typing(t, repo.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "passed (1 of 1 finished)")

	if rung := repo.asked("notify"); len(rung) != 1 {
		t.Errorf("notify calls = %v, want one ring, when the re-run passed", rung)
	}
}

func TestAReRunThatFailsAgainSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// The re-run fails too, under an id of its own.
	repo := newWorld()
	repo.ciInterval = pollTick
	repo.ci = []forge.CI{failedRun("11"), failedRun("12")}

	// Act
	view := typing(t, repo.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "CI     ✗ failed")
}

func TestAFailedStatusNotReRunDoesNotKeepCIRunning(t *testing.T) {
	t.Parallel()

	// Arrange
	// A status reported by something other than the forge's runs fails beside
	// the run that failed, and a run that passed; only the run is re-run. The
	// re-run passes, and the status fails on as it did.
	status := forge.Check{Name: "ci/external", State: forge.CIFailed}
	passed := forge.Check{ID: "10", Name: "go-vet", State: forge.CIPassed, LogAvailable: true}
	first := failedRun("11")
	first.Total, first.Done, first.Checks = 3, 3, append(first.Checks, passed, status)
	rerun := forge.Check{ID: "12", Name: rerunCheck, State: forge.CIPassed, LogAvailable: true}
	repo := newWorld()
	repo.ciInterval = pollTick
	repo.ci = []forge.CI{
		first,
		{State: forge.CIFailed, Total: 3, Done: 3, Failed: 1, Checks: []forge.Check{rerun, passed, status}},
	}

	// Act
	view := typing(t, repo.live(t, 120, 40), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "CI     ✗ failed", "ci/external")
}

func TestAFailedRunTheForgeDoesNotReRunSettlesOnTheFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two runs failed; the forge re-runs one, which passes under an id of its
	// own, and leaves the other failed as it was. Each read of CI comes as
	// long after the one before as the default interval asks.
	left := forge.Check{ID: "13", Name: "e2e", State: forge.CIFailed, LogAvailable: true}
	first := failedRun("11")
	first.Total, first.Done, first.Failed, first.Checks = 2, 2, 2, append(first.Checks, left)
	rerun := forge.Check{ID: "12", Name: rerunCheck, State: forge.CIPassed, LogAvailable: true}
	repo := newWorld()
	repo.ciInterval = pollTick
	repo.ci = []forge.CI{
		first,
		{State: forge.CIFailed, Total: 2, Done: 2, Failed: 1, Checks: []forge.Check{rerun, left}},
	}
	deps := repo.deps()
	check := deps.Forge.CheckStatus
	deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		repo.mu.Lock()
		repo.elapsed += 20 * time.Second
		repo.mu.Unlock()

		return check(ctx, pull, head)
	}

	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "4", "R", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "CI     ✗ failed (2 of 2 finished)", "e2e")
}
