// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The harness that drives the interface in tests: run a model to quiescence,
// type keys, click, and read the screen. The world it drives lives in
// world_test.go.

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// horizon is how far a drain lets fake time run: past the 150 ms a selection
// rests before its issue is read and the millisecond CI polls a test asks for,
// and short of the two-second interval a test sets to keep CI from being asked
// again before its next key, and the twenty-second default. A wait due later
// never fires.
const horizon = time.Second

// failsafe is how long a drain may take on the wall clock before the test fails.
// Every fake answers at once and every wait is on the fake clock, so only a fake
// left blocked, or a model that never stops asking, can reach it — and either is
// a broken test to be told about, not a slow command to drop.
const failsafe = 10 * time.Second

// scheduled is a wait the interface asked the fake timer for: fire's message,
// once after has passed.
type scheduled struct {
	after time.Duration
	fire  func(time.Time) tea.Msg
}

// fakeAfter is the timer the tests hand the interface. It spends no time: its
// command hands the wait back for the drain to run on a fake clock.
func fakeAfter(wait time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
	return func() tea.Msg { return scheduled{after: wait, fire: fire} }
}

// timer is a wait on a drain's fake clock, due some way into the drain.
type timer struct {
	due  time.Duration
	fire func(time.Time) tea.Msg
}

// fakeClock is the time a drain lets pass: how far in it has run, and the waits
// still to fire, soonest first.
type fakeClock struct {
	elapsed time.Duration
	waits   []timer
}

// schedule puts a wait on the clock, after every wait due no later, so two waits
// due together fire in the order they were asked for.
func (c *fakeClock) schedule(wait scheduled) {
	due := c.elapsed + wait.after

	at := slices.IndexFunc(c.waits, func(queued timer) bool { return queued.due > due })
	if at < 0 {
		at = len(c.waits)
	}

	c.waits = slices.Insert(c.waits, at, timer{due: due, fire: wait.fire})
}

// next moves the clock on to the soonest wait and returns the command that fires
// it, or reports that no wait falls due before horizon.
func (c *fakeClock) next() (tea.Cmd, bool) {
	if len(c.waits) == 0 || c.waits[0].due > horizon {
		return nil, false
	}

	wait := c.waits[0]
	c.waits = c.waits[1:]
	c.elapsed = wait.due
	at := testNow().Add(wait.due)

	return func() tea.Msg { return wait.fire(at) }, true
}

// drain runs a command and every command it leads to, one at a time in the order
// they were made, delivering each message the way Bubble Tea would. A wait the
// interface asked the fake timer for fires once nothing else is left to run,
// soonest first, until the next one falls past horizon.
func drain(t *testing.T, model tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()

	deadline := time.NewTimer(failsafe)
	defer deadline.Stop()

	var clock fakeClock

	pending := []tea.Cmd{cmd}

	for {
		if len(pending) == 0 {
			fire, due := clock.next()
			if !due {
				return model
			}

			pending = append(pending, fire)
		}

		next := pending[0]
		pending = pending[1:]

		if next == nil {
			continue
		}

		switch msg := await(t, next, deadline.C).(type) {
		case tea.BatchMsg:
			pending = append(pending, msg...)
		case scheduled:
			clock.schedule(msg)
		default:
			updated, follow := model.Update(msg)
			model = concrete(t, updated)

			pending = append(pending, follow)
		}
	}
}

// await runs a command and returns its message, failing the test if the drain's
// deadline passes first.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func await(t *testing.T, cmd tea.Cmd, deadline <-chan time.Time) tea.Msg {
	t.Helper()

	answer := make(chan tea.Msg, 1)

	go func() { answer <- cmd() }()

	select {
	case msg := <-answer:
		return msg
	case <-deadline:
		t.Fatalf("the model had not settled after %v: a fake is blocked, or the model never stops asking", failsafe)

		return nil
	}
}

// live is the world's interface, sized and with everything it loads at start
// loaded.
func (w *world) live(t *testing.T, width, height int) tui.Model {
	t.Helper()

	model := sized(t, tui.New(w.cfg, nil, w.deps()), width, height)

	return drain(t, model, model.Init())
}

// typing presses keys in order, finishing whatever each one starts.
func typing(t *testing.T, model tui.Model, keys ...string) tui.Model {
	t.Helper()

	for _, key := range keys {
		updated, cmd := model.Update(keyMsg(key))
		model = drain(t, concrete(t, updated), cmd)
	}

	return model
}

// letters is each character of text as its own key.
func letters(text string) []string {
	keys := make([]string, 0, len(text))
	for _, character := range text {
		keys = append(keys, string(character))
	}

	return keys
}

// footerLine is the last line of a screen, with its color escapes stripped so a
// test can match the keys it names as plain text.
func footerLine(view string) string {
	lines := strings.Split(view, "\n")

	return ansi.Strip(lines[len(lines)-1])
}

// plain is a screen or line with its color escapes stripped, for a test that
// matches it as plain text without going through requireScreen.
func plain(view string) string {
	return ansi.Strip(view)
}

// requireScreen fails the test unless the screen shows every one of want. The
// color escapes lipgloss v2 renders into the string are stripped first, so a
// wanted phrase matches whether or not it is styled — the color tests assert on
// the escapes through their own raw-output helpers instead.
func requireScreen(t *testing.T, view string, want ...string) {
	t.Helper()

	plain := ansi.Strip(view)

	for _, each := range want {
		if !strings.Contains(plain, each) {
			t.Errorf("the screen does not show %q:\n%s", each, plain)
		}
	}
}

// refuseScreen fails the test if the screen shows any of unwanted, matched
// against the screen with its color escapes stripped.
func refuseScreen(t *testing.T, view string, unwanted ...string) {
	t.Helper()

	plain := ansi.Strip(view)

	for _, each := range unwanted {
		if strings.Contains(plain, each) {
			t.Errorf("the screen shows %q:\n%s", each, plain)
		}
	}
}

// click is a left click at a cell, and whatever it starts, finished.
func click(t *testing.T, model tui.Model, column, row int) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseClickMsg{X: column, Y: row, Button: tea.MouseLeft})

	return drain(t, concrete(t, updated), cmd)
}

// commitKeys opens the composer on the Commits pane, types a subject and
// commits, then presses any more keys.
func commitKeys(subject string, more ...string) []string {
	keys := append(append([]string{"3", "c"}, letters(subject)...), keyEnter)

	return append(keys, more...)
}
