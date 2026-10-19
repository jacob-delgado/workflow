// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

// A small synchronous driver for the real terminal interface over the demo
// world: it runs every command the interface makes in the test's own
// goroutine, in order, and fires the waits it asks for on a fake clock that
// the world's clock reads too, so CI moves on as the test lets time pass.

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// settleWithin is how far a drain lets fake time run after a key: past the
// rest a selection takes before its issue is read, short of the demo's CI
// interval, so CI is asked again only when a test waits for it.
const settleWithin = time.Second

// The size the tests draw the interface at: roomy enough that no fixture is
// cut short.
const (
	screenWidth  = 140
	screenHeight = 44
)

// fakeTime is how long the test has let pass since the world began, and the
// waits the interface asked for, soonest first.
type fakeTime struct {
	elapsed time.Duration
	waits   []timer
}

// since is the world's elapsed time.
func (c *fakeTime) since() time.Duration {
	return c.elapsed
}

// timer is a wait due at some elapsed time.
type timer struct {
	due  time.Duration
	fire func(time.Time) tea.Msg
}

// scheduled is a wait the interface asked the fake timer for.
type scheduled struct {
	after time.Duration
	fire  func(time.Time) tea.Msg
}

// fakeAfter is the timer the tests hand the interface: its command hands the
// wait back to the drain rather than sleeping.
func fakeAfter(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	return func() tea.Msg { return scheduled{after: wait, fire: fire} }
}

// session is the demo world and the interface over it, driven by a test.
type session struct {
	t     *testing.T
	clock *fakeTime
	world *demo.World
	model tui.Model
}

// opened is the demo's interface, sized, with motion held still and every
// load it starts with finished.
func opened(t *testing.T) *session {
	t.Helper()

	clock := &fakeTime{}
	world := demo.New(clock.since)

	deps := world.Deps()
	deps.After = fakeAfter

	cfg := world.Config()
	cfg.UI.Motion = config.MotionReduce

	run := &session{t: t, clock: clock, world: world, model: tui.New(cfg, nil, deps)}
	run.deliver(tea.WindowSizeMsg{Width: screenWidth, Height: screenHeight})
	run.drain(run.model.Init(), settleWithin)

	return run
}

// press sends each key in turn, finishing whatever it starts.
func (s *session) press(keys ...string) {
	s.t.Helper()

	for _, pressed := range keys {
		s.drain(s.deliver(keyPress(pressed)), settleWithin)
	}
}

// screen is the interface as drawn, its color escapes stripped.
func (s *session) screen() string {
	return ansi.Strip(s.model.View().Content)
}

// requireScreen fails the test unless screen shows every one of want.
func requireScreen(t *testing.T, screen string, want ...string) {
	t.Helper()

	for _, each := range want {
		if !strings.Contains(screen, each) {
			t.Errorf("the screen does not show %q:\n%s", each, screen)
		}
	}
}

// deliver hands msg to the interface and returns what it asks to run next.
func (s *session) deliver(msg tea.Msg) tea.Cmd {
	s.t.Helper()

	updated, follow := s.model.Update(msg)

	model, isModel := updated.(tui.Model)
	if !isModel {
		s.t.Fatalf("Update returned %T, want tui.Model", updated)
	}

	s.model = model

	return follow
}

// drain runs cmd and everything it leads to, firing the waits due within
// window of now, soonest first, once nothing else is left to run.
func (s *session) drain(cmd tea.Cmd, window time.Duration) {
	s.t.Helper()

	until := s.clock.elapsed + window
	pending := []tea.Cmd{cmd}

	for {
		if len(pending) == 0 {
			fire, due := s.nextWait(until)
			if !due {
				return
			}

			pending = append(pending, fire)
		}

		next := pending[0]
		pending = pending[1:]

		if next != nil {
			pending = s.run(next, pending)
		}
	}
}

// run runs one command and returns what is left to run after it.
func (s *session) run(next tea.Cmd, pending []tea.Cmd) []tea.Cmd {
	s.t.Helper()

	msg := next()
	if sequenced, isSequence := sequence(msg); isSequence {
		return append(sequenced, pending...)
	}

	switch msg := msg.(type) {
	case nil:
		return pending
	case tea.BatchMsg:
		return append(pending, msg...)
	case scheduled:
		s.schedule(msg)

		return pending
	default:
		return append(pending, s.deliver(msg))
	}
}

// schedule puts a wait on the clock after every wait due no later.
func (s *session) schedule(wait scheduled) {
	due := s.clock.elapsed + wait.after

	at := slices.IndexFunc(s.clock.waits, func(queued timer) bool { return queued.due > due })
	if at < 0 {
		at = len(s.clock.waits)
	}

	s.clock.waits = slices.Insert(s.clock.waits, at, timer{due: due, fire: wait.fire})
}

// nextWait moves the clock on to the soonest wait due by until and returns the
// command that fires it, or reports that none is due.
func (s *session) nextWait(until time.Duration) (tea.Cmd, bool) {
	if len(s.clock.waits) == 0 || s.clock.waits[0].due > until {
		return nil, false
	}

	wait := s.clock.waits[0]
	s.clock.waits = s.clock.waits[1:]
	s.clock.elapsed = max(s.clock.elapsed, wait.due)
	at := s.world.Deps().Clock()

	return func() tea.Msg { return wait.fire(at) }, true
}

// sequence is the commands a tea.Sequence message runs in order, and whether
// msg is one: Bubble Tea keeps its type unexported, so it is told by shape.
func sequence(msg tea.Msg) ([]tea.Cmd, bool) {
	if _, batch := msg.(tea.BatchMsg); batch {
		return nil, false
	}

	value := reflect.ValueOf(msg)
	if value.Kind() != reflect.Slice || value.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return nil, false
	}

	cmds := make([]tea.Cmd, value.Len())
	for index := range cmds {
		cmds[index], _ = reflect.TypeAssert[tea.Cmd](value.Index(index))
	}

	return cmds, true
}

// keyPress is the press of the key named, as Bubble Tea delivers it.
func keyPress(named string) tea.KeyPressMsg {
	special := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
		"down": tea.KeyDown, "up": tea.KeyUp,
	}

	if code, isSpecial := special[named]; isSpecial {
		return tea.KeyPressMsg{Code: code}
	}

	return tea.KeyPressMsg{Code: rune(named[0]), Text: named}
}
