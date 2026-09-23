// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The interface's waits — the selection resting before an issue is read, and
// the gap between CI checks — go through the timer it was given, so a test can
// run them on a clock of its own rather than the wall's.

import (
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// recordingTimer is an After that notes each wait it is asked for and fires at
// once, so a test sees the waits without spending them.
type recordingTimer struct {
	waits []time.Duration
}

// after notes the wait and fires straight away.
func (r *recordingTimer) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	r.waits = append(r.waits, wait)

	return func() tea.Msg { return fire(testNow().Add(wait)) }
}

// timed is the world's interface, started, with its waits going to timer.
func timed(t *testing.T, repo *world, timer *recordingTimer) tui.Model {
	t.Helper()

	deps := repo.deps()
	deps.After = timer.after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	return drain(t, model, model.Init())
}

func TestTheSelectionRestsOnTheTimerItWasGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	timer := &recordingTimer{}
	screen := timed(t, newWorld(), timer)
	timer.waits = nil

	// Act
	typing(t, screen, "j")

	// Assert
	if want := []time.Duration{150 * time.Millisecond}; !slices.Equal(timer.waits, want) {
		t.Errorf("moving the selection waited %v on the timer, want %v", timer.waits, want)
	}
}

func TestCIIsAskedAgainOnTheTimerItWasGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI runs at the first check and has passed by the next, asked about every
	// seven seconds.
	running := newWorld()
	running.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	running.ciInterval = 7 * time.Second
	timer := &recordingTimer{}

	// Act
	timed(t, running, timer)

	// Assert
	if want := []time.Duration{7 * time.Second}; !slices.Equal(timer.waits, want) {
		t.Errorf("CI polling waited %v on the timer, want %v", timer.waits, want)
	}

	if checks := running.asked("ci"); len(checks) != 2 {
		t.Errorf("CI was checked %d times, want twice: once running, once passed", len(checks))
	}
}
