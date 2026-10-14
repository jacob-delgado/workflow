// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// stepper is a timer that keeps every wait the interface asks for rather than
// letting a drain fire it, so a test moves the interface on one frame at a
// time.
type stepper struct {
	mu    sync.Mutex
	waits []scheduled
}

// after keeps the wait for step.
func (s *stepper) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.waits = append(s.waits, scheduled{after: wait, fire: fire})

	return nil
}

// step fires the wait asked for last, as its time arrives, and finishes
// whatever it starts.
func (s *stepper) step(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	s.mu.Lock()
	if len(s.waits) == 0 {
		s.mu.Unlock()
		t.Fatal("the interface asked for no frame")
	}

	wait := s.waits[len(s.waits)-1]
	s.waits = s.waits[:len(s.waits)-1]
	s.mu.Unlock()

	next, cmd := model.Update(wait.fire(testNow().Add(wait.after)))

	return drain(t, concrete(t, next), cmd)
}

// counter is the drains' own fake timer, counting the waits asked of it.
type counter struct{ asked atomic.Int32 }

// after counts the wait and hands it to the drain's clock.
func (c *counter) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	c.asked.Add(1)

	return fakeAfter(wait, fire)
}

// moving is the world with motion back on, as a user's file leaves it.
func moving() *world {
	repo := newWorld()
	repo.cfg.UI.Motion = ""

	return repo
}

// heldReview is deps whose CI check waits on held, and whose timer is after.
func heldReview(after func(time.Duration, func(time.Time) tea.Msg) tea.Cmd) func(*tui.Deps, *hold) {
	return func(deps *tui.Deps, held *hold) {
		deps.After = after
		check := deps.Forge.CheckStatus
		deps.Forge.CheckStatus = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
			held.wait()

			return check(ctx, pull, head)
		}
	}
}

func TestTheInFlightMarkTurnsWhileAReadIsOut(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}
	held := newHold(t)
	started := heldLive(t, held, moving(), heldReview(frames.after))
	held.armed.Store(true)
	refreshing := holding(t, held, started, "4", "r")

	// Act
	turned := frames.step(t, refreshing)

	// Assert
	requireScreen(t, turned.View().Content, "Review ◓")
}

func TestReducedMotionKeepsTheInFlightMarkStill(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}
	held := newHold(t)
	started := heldLive(t, held, newWorld(), heldReview(frames.after))
	held.armed.Store(true)

	// Act
	view := holding(t, held, started, "4", "r").View().Content

	// Assert
	requireScreen(t, view, "Review ◐")

	if len(frames.waits) != 0 {
		t.Errorf("asked for %d frames under reduced motion, want none", len(frames.waits))
	}
}

func TestNothingTicksWhileNothingMoves(t *testing.T) {
	t.Parallel()

	// Arrange
	timer := &counter{}
	repo := moving()
	deps := repo.deps()
	deps.After = timer.after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)
	idle := drain(t, model, model.Init())
	before := timer.asked.Load()

	// Act
	typing(t, idle, "?")

	// Assert
	if asked := timer.asked.Load() - before; asked != 0 {
		t.Errorf("asked for %d waits with nothing moving, want none", asked)
	}
}
