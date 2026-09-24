// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The interface's waits — the selection resting before an issue is read, and
// the gap between CI checks — go through the timer it was given, so a test can
// run them on a clock of its own rather than the wall's. Given none, as the
// shipped binary is, they fall to Bubble Tea's own tea.Tick on the wall clock.

import (
	"slices"
	"sync"
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

// heldTimer is an After that holds every wait until the test releases it, so a
// wait can outlive the keys typed while it is due: a poll scheduled on one branch
// still fires after a switch away and back, as it would on the wall clock.
type heldTimer struct {
	mu    sync.Mutex
	fires []func(time.Time) tea.Msg
}

// after holds the wait once its command runs. The command yields nothing, so the
// drain that asked for the wait finishes without it.
func (h *heldTimer) after(_ time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		h.mu.Lock()
		defer h.mu.Unlock()

		h.fires = append(h.fires, fire)

		return nil
	}
}

// waiting is how many waits are held.
func (h *heldTimer) waiting() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.fires)
}

// release fires every wait held so far against model, each finished before the
// next — as waits falling due apart would be — and holds any wait the firing asks
// for anew.
func (h *heldTimer) release(t *testing.T, model tui.Model) {
	t.Helper()

	h.mu.Lock()
	due := h.fires
	h.fires = nil
	h.mu.Unlock()

	for _, fire := range due {
		model = drain(t, model, func() tea.Msg { return fire(testNow()) })
	}
}

// timed is the world's interface, started, with its waits going to after.
func timed(t *testing.T, repo *world, after func(time.Duration, func(time.Time) tea.Msg) tea.Cmd) tui.Model {
	t.Helper()

	deps := repo.deps()
	deps.After = after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	return drain(t, model, model.Init())
}

func TestTheSelectionRestsOnTheTimerItWasGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	timer := &recordingTimer{}
	screen := timed(t, newWorld(), timer.after)
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
	timed(t, running, timer.after)

	// Assert
	if want := []time.Duration{7 * time.Second}; !slices.Equal(timer.waits, want) {
		t.Errorf("CI polling waited %v on the timer, want %v", timer.waits, want)
	}

	if checks := running.asked("ci"); len(checks) != 2 {
		t.Errorf("CI was checked %d times, want twice: once running, once passed", len(checks))
	}
}

func TestWithNoTimerGivenCIIsAskedAgainOnTheWallClock(t *testing.T) {
	t.Parallel()

	// Arrange
	// No After, as the wiring builds it: the poll waits on the wall clock, and a
	// short interval keeps that wait brief.
	running := newWorld()
	running.ci = []forge.CI{{State: forge.CIRunning}, {State: forge.CIPassed}}
	running.ciInterval = pollTick
	deps := running.deps()
	deps.After = nil
	model := sized(t, tui.New(running.cfg, nil, deps), 120, 40)

	// Act
	drain(t, model, model.Init())

	// Assert
	if checks := running.asked("ci"); len(checks) != 2 {
		t.Errorf("CI was checked %d times on the default timer, want twice: once running, once passed", len(checks))
	}
}
