// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// stepper is a timer that keeps every wait the interface asks for rather than
// letting a drain fire it, so a test moves the interface on one frame at a
// time. Like a drain, it drops a wait due past horizon, which never fires.
type stepper struct {
	mu    sync.Mutex
	waits []scheduled
}

// after keeps the wait for step.
func (s *stepper) after(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	if wait > horizon {
		return nil
	}

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

// settleFrames fires every frame the interface goes on asking for, until it
// asks for none, and fails the test if it never stops.
func (s *stepper) settleFrames(t *testing.T, model tui.Model) tui.Model {
	t.Helper()

	const most = 100

	for range most {
		s.mu.Lock()
		left := len(s.waits)
		s.mu.Unlock()

		if left == 0 {
			return model
		}

		model = s.step(t, model)
	}

	t.Fatalf("still asking for frames after %d", most)

	return model
}

// reviewRule is the rule after the Review stage, plain.
func reviewRule(view string) string {
	_, after, _ := strings.Cut(ansi.Strip(spineLine(view)), " Review ")
	rule, _, _ := strings.Cut(after, " ")

	return rule
}

// passingDuringTheSession is a world whose CI is running when the session
// opens and has passed by the time Review is refreshed, on the timer frames.
// It polls CI past the horizon, so only the refresh asks again.
func passingDuringTheSession(t *testing.T, repo *world, frames *stepper) tui.Model {
	t.Helper()

	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{
		{State: forge.CIRunning, Total: 4, Done: 0},
		{State: forge.CIPassed, Total: 4, Done: 4},
	}
	deps := repo.deps()
	deps.After = frames.after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)
	model = frames.settleFrames(t, drain(t, model, model.Init()))

	return typing(t, model, "4", "r")
}

func TestACompletingStagesRuleFillsOverFrames(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}
	passed := passingDuringTheSession(t, moving(), frames)

	// Act: one frame on
	stepped := frames.step(t, passed)

	// Assert: part filled
	rule := reviewRule(stepped.View().Content)
	if !strings.Contains(rule, filledRule) || !strings.Contains(rule, openRule) {
		t.Errorf("Review's rule after one frame = %q, want it part filled", rule)
	}

	// Act: every frame it asks for
	settled := frames.settleFrames(t, stepped)

	// Assert: filled
	if rule := reviewRule(settled.View().Content); strings.Contains(rule, openRule) {
		t.Errorf("Review's rule once settled = %q, want it filled", rule)
	}
}

func TestReducedMotionFillsACompletingStagesRuleAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}

	// Act
	passed := passingDuringTheSession(t, newWorld(), frames)

	// Assert
	if rule := reviewRule(passed.View().Content); strings.Contains(rule, openRule) || rule == "" {
		t.Errorf("Review's rule = %q, want it filled at once", rule)
	}
}

func TestTheTrackDrawsItsFirstScreenFilledWithoutMoving(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}
	repo := moving()
	deps := repo.deps()
	deps.After = frames.after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)

	// Act
	opened := drain(t, model, model.Init())

	// Assert
	if rule := reviewRule(opened.View().Content); strings.Contains(rule, openRule) {
		t.Errorf("Review's rule on the first screen = %q, want it already filled", rule)
	}
}

// ciBar is the Review detail's bar of checks finished, plain: the row after
// the CI line, from the CI line's column on, its rules only.
func ciBar(t *testing.T, view string) string {
	t.Helper()

	lines := strings.Split(ansi.Strip(view), "\n")
	for index, line := range lines {
		before, _, found := strings.Cut(line, " CI ")
		if found && index+1 < len(lines) {
			under := []rune(lines[index+1])[len([]rune(before)):]

			return strings.Map(func(r rune) rune {
				if string(r) == filledRule || string(r) == openRule {
					return r
				}

				return -1
			}, string(under))
		}
	}

	t.Fatalf("no CI line on the screen:\n%s", view)

	return ""
}

func TestTheReviewBarSpringsTowardTheChecksFinished(t *testing.T) {
	t.Parallel()

	// Arrange
	frames := &stepper{}
	repo := moving()
	repo.ciInterval = 2 * time.Second
	repo.ci = []forge.CI{
		{State: forge.CIRunning, Total: 12, Done: 3},
		{State: forge.CIRunning, Total: 12, Done: 7},
	}
	deps := repo.deps()
	deps.After = frames.after
	model := sized(t, tui.New(repo.cfg, nil, deps), 120, 40)
	model = frames.settleFrames(t, drain(t, model, model.Init()))
	before := strings.Count(ciBar(t, typing(t, model, "4").View().Content), filledRule)
	refreshed := typing(t, model, "4", "r")

	// Act: one frame on
	stepped := frames.step(t, refreshed)

	// Assert: on its way from three of twelve toward seven
	view := stepped.View().Content
	bar := ciBar(t, view)

	if filled := strings.Count(bar, filledRule); filled <= before || filled*12 >= 7*len([]rune(bar)) {
		t.Errorf("the bar after one frame = %q, want it between 3 and 7 of 12 filled", bar)
	}

	requireScreen(t, view, "7 of 12 finished")

	// Act: every frame it asks for
	settled := frames.settleFrames(t, stepped)

	// Assert: seven of twelve
	bar = ciBar(t, settled.View().Content)
	if filled := strings.Count(bar, filledRule); filled*12 != 7*len([]rune(bar)) {
		t.Errorf("the bar once settled = %q, want 7 of 12 filled", bar)
	}
}
