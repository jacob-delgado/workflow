// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// pollTick is a CI interval short enough that the poll fires within a test's
// patience, so a running-then-finished sequence plays out without a real wait.
const pollTick = 5 * time.Millisecond

// finishingCI is a world whose CI is running on the first check and finished on
// the next, polled fast enough to see the change within the test.
func finishingCI(finished forge.CIState) *world {
	repo := newWorld()
	repo.ci = []forge.CI{
		{State: forge.CIRunning, Total: 1, Done: 0},
		{State: finished, Total: 1, Done: 1},
	}
	repo.ciInterval = pollTick

	return repo
}

func TestCIFinishingRingsWhenNotifyIsOn(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := finishingCI(forge.CIPassed)
	cfg := completeConfig()
	cfg.UI.Notify = true
	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if rung := repo.asked("notify"); len(rung) != 1 {
		t.Errorf("notify calls = %v, want one ring when CI finished", rung)
	}
}

func TestCIFailingAlsoRings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := finishingCI(forge.CIFailed)
	cfg := completeConfig()
	cfg.UI.Notify = true
	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if rung := repo.asked("notify"); len(rung) != 1 {
		t.Errorf("notify calls = %v, want one ring when CI failed", rung)
	}
}

func TestCIFinishingIsSilentWhenNotifyIsOff(t *testing.T) {
	t.Parallel()

	// Arrange
	// completeConfig leaves ui.notify off.
	repo := finishingCI(forge.CIPassed)
	model := sized(t, tui.New(completeConfig(), nil, repo.deps()), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if rung := repo.asked("notify"); len(rung) != 0 {
		t.Errorf("notify calls = %v, want none with the setting off", rung)
	}
}

func TestCIAlreadyFinishedDoesNotRing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The default world's CI is already passed on the first check, so there is
	// no change from running to report.
	repo := newWorld()
	cfg := completeConfig()
	cfg.UI.Notify = true
	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if rung := repo.asked("notify"); len(rung) != 0 {
		t.Errorf("notify calls = %v, want none when CI was never seen running", rung)
	}
}

func TestNotifyPollsOnALongerBeatWithNoIntervalSet(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI stays running and no interval is configured, so the notification poll
	// falls back to its slower beat rather than ringing anything.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning, Total: 1, Done: 0}}
	cfg := completeConfig()
	cfg.UI.Notify = true
	model := sized(t, tui.New(cfg, nil, repo.deps()), 120, 40)

	// Act
	settled := drain(t, model, model.Init())

	// Assert
	requireScreen(t, settled.View().Content, "running")

	if rung := repo.asked("notify"); len(rung) != 0 {
		t.Errorf("notify calls = %v, want none while CI is still running", rung)
	}
}
