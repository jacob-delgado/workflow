// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The harness that drives the interface in tests: run a model to quiescence,
// type keys, click, and read the screen. The world it drives lives in
// world_test.go.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// drain runs a command and every command it leads to, delivering each message,
// the way Bubble Tea would — until nothing is left, or what is left is a timer
// longer than patience.
func drain(t *testing.T, model tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()

	pending := []tea.Cmd{cmd}

	for steps := 0; len(pending) > 0 && steps < 300; steps++ {
		next := pending[0]
		pending = pending[1:]

		if next == nil {
			continue
		}

		msg, arrived := within(next)
		if !arrived {
			continue
		}

		if batch, isBatch := msg.(tea.BatchMsg); isBatch {
			pending = append(pending, batch...)

			continue
		}

		updated, follow := model.Update(msg)
		model = concrete(t, updated)

		pending = append(pending, follow)
	}

	return model
}

// within runs a command, giving up after patience.
//
//nolint:ireturn // tea.Msg is Bubble Tea's type for any message at all
func within(cmd tea.Cmd) (tea.Msg, bool) {
	answer := make(chan tea.Msg, 1)

	go func() { answer <- cmd() }()

	select {
	case msg := <-answer:
		return msg, true
	case <-time.After(patience):
		return nil, false
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

// footerLine is the last line of a screen.
func footerLine(view string) string {
	lines := strings.Split(view, "\n")

	return lines[len(lines)-1]
}

// requireScreen fails the test unless the screen shows every one of want.
func requireScreen(t *testing.T, view string, want ...string) {
	t.Helper()

	for _, each := range want {
		if !strings.Contains(view, each) {
			t.Errorf("the screen does not show %q:\n%s", each, view)
		}
	}
}

// refuseScreen fails the test if the screen shows any of unwanted.
func refuseScreen(t *testing.T, view string, unwanted ...string) {
	t.Helper()

	for _, each := range unwanted {
		if strings.Contains(view, each) {
			t.Errorf("the screen shows %q:\n%s", each, view)
		}
	}
}

// click is a left click at a cell, and whatever it starts, finished.
func click(t *testing.T, model tui.Model, column, row int) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseMsg{
		X: column, Y: row, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})

	return drain(t, concrete(t, updated), cmd)
}

// commitKeys opens the composer on the Commits pane, types a subject and
// commits, then presses any more keys.
func commitKeys(subject string, more ...string) []string {
	keys := append(append([]string{"3", "c"}, letters(subject)...), keyEnter)

	return append(keys, more...)
}
