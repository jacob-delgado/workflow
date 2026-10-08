// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The harness that drives the interface in tests: run a model to quiescence,
// type keys, click, and read the screen. The world it drives lives in
// world_test.go.

import (
	"slices"
	"strings"
	"sync/atomic"
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

	return settle(t, model, cmd, driver{
		run: func(next tea.Cmd) (tea.Msg, bool) { return await(t, next, deadline.C), true },
		deliver: func(model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
			updated, follow := model.Update(msg)

			return concrete(t, updated), follow
		},
	})
}

// drainPast is drain leaving out every command held keeps from answering: a
// seam the test has armed held on, so that it never answers, while the screen is
// looked at with its answer still out. Any other command is waited for as drain
// waits, so a slow fake is never mistaken for a held one, and a message the model
// itself waits on held for fails the test.
func drainPast(t *testing.T, held *hold, model tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()

	deadline := time.NewTimer(failsafe)
	defer deadline.Stop()

	return settle(t, model, cmd, driver{
		run: func(next tea.Cmd) (tea.Msg, bool) {
			answer := make(chan tea.Msg, 1)

			go func() { answer <- next() }()

			select {
			case msg := <-answer:
				return msg, true
			case <-held.waiting:
				return nil, false
			case <-deadline.C:
				t.Fatalf("the model had not settled after %v: a fake is blocked, or the model never stops asking", failsafe)

				return nil, false
			}
		},
		deliver: func(model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) { return held.update(t, model, msg) },
	})
}

// driver is how a drain runs a command for its message, reporting whether it
// answered, and how it hands a message to the model.
type driver struct {
	run     func(tea.Cmd) (tea.Msg, bool)
	deliver func(tui.Model, tea.Msg) (tui.Model, tea.Cmd)
}

// settle runs a command and every command it leads to, as drain describes,
// with drive; a command it leaves unanswered leads to nothing.
func settle(t *testing.T, model tui.Model, cmd tea.Cmd, drive driver) tui.Model {
	t.Helper()

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

		msg, answered := drive.run(next)
		if !answered {
			continue
		}

		switch msg := msg.(type) {
		case tea.BatchMsg:
			pending = append(pending, msg...)
		case scheduled:
			clock.schedule(msg)
		default:
			var follow tea.Cmd

			model, follow = drive.deliver(model, msg)
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

// hold keeps a seam from answering once it is armed, as a service that has
// stopped answering does, and says so each time a call starts waiting on it, so
// a drain leaves out exactly the commands it holds; it counts those calls, so a
// test can tell a held seam was reached at all, and the test's cleanup lets every
// held answer go.
type hold struct {
	armed   atomic.Bool
	reached atomic.Int32
	waiting chan struct{}
	release chan struct{}
}

// newHold is a hold not yet armed, released when t ends.
func newHold(t *testing.T) *hold {
	t.Helper()

	held := &hold{waiting: make(chan struct{}), release: make(chan struct{})}

	t.Cleanup(func() { close(held.release) })

	return held
}

// wait returns at once until the hold is armed, and after that only once the
// test has ended, having told the drain running its call that it waits.
func (h *hold) wait() {
	if !h.armed.Load() {
		return
	}

	h.reached.Add(1)

	select {
	case h.waiting <- struct{}{}:
	case <-h.release:
	}

	<-h.release
}

// requireReached fails the test unless a call has started waiting on the armed
// hold, so a screen looked at "while the answer is out" proves an answer was
// asked for at all.
func (h *hold) requireReached(t *testing.T) {
	t.Helper()

	if h.reached.Load() == 0 {
		t.Fatal("no call reached the held seam: what was pressed asked it for nothing")
	}
}

// update hands msg to model as Bubble Tea would, failing the test when the
// model itself waits on the hold to deal with it: a key or an answer is dealt
// with at once, and whatever has to wait on a service goes in a command.
func (h *hold) update(t *testing.T, model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	t.Helper()

	type updated struct {
		model  tea.Model
		follow tea.Cmd
	}

	answer := make(chan updated, 1)

	go func() {
		next, follow := model.Update(msg)
		answer <- updated{model: next, follow: follow}
	}()

	select {
	case got := <-answer:
		return concrete(t, got.model), got.follow
	case <-h.waiting:
		t.Fatalf("the model waited on a held seam itself to deal with %T; that belongs in a command", msg)
	case <-time.After(failsafe):
		t.Fatalf("the model had not dealt with %T after %v", msg, failsafe)
	}

	return model, nil
}

// holding presses keys in order, as typing does, leaving out whatever held keeps
// from answering.
func holding(t *testing.T, held *hold, model tui.Model, keys ...string) tui.Model {
	t.Helper()

	for _, key := range keys {
		updated, cmd := held.update(t, model, keyMsg(key))
		model = drainPast(t, held, updated, cmd)
	}

	return model
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
