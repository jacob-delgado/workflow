// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Two reads of the same thing can be out at once, and a service answers them
// in whatever order it likes. withholding keeps a read's answer back, so a
// test can hand it over after a later read's, as a slow answer arrives.

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// withholding runs cmd and every command it leads to, as drain does, but
// keeps back the answer of each command during which calls grew — a seam's
// calls, counted — and returns those answers, undelivered, in the order they
// came. Commands run one at a time, so a count that grew while one ran grew
// for that one.
func withholding(t *testing.T, model tui.Model, cmd tea.Cmd, calls func() int) (tui.Model, []tea.Msg) {
	t.Helper()

	deadline := time.NewTimer(failsafe)
	defer deadline.Stop()

	var kept []tea.Msg

	model = settle(t, model, cmd, driver{
		run: func(next tea.Cmd) (tea.Msg, bool) {
			before := calls()
			msg := await(t, next, deadline.C)

			if calls() > before {
				kept = append(kept, msg)

				return nil, false
			}

			return msg, true
		},
		deliver: func(model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
			updated, follow := model.Update(msg)

			return concrete(t, updated), follow
		},
	})

	return model, kept
}

// withheldKeys types keys as typing does, keeping back each answer of a
// command during which calls grew, and returns those answers.
func withheldKeys(t *testing.T, model tui.Model, calls func() int, keys ...string) (tui.Model, []tea.Msg) {
	t.Helper()

	var kept []tea.Msg

	for _, key := range keys {
		updated, cmd := model.Update(stepMsg(key))

		var answers []tea.Msg

		model, answers = withholding(t, concrete(t, updated), cmd, calls)
		kept = append(kept, answers...)
	}

	return model, kept
}

// handedOver delivers each answer a test kept back, and every command each
// leads to, in order.
func handedOver(t *testing.T, model tui.Model, answers []tea.Msg) tui.Model {
	t.Helper()

	for _, answer := range answers {
		model = drain(t, model, func() tea.Msg { return answer })
	}

	return model
}

// counting is how many calls the world was asked starting with prefix, as a
// count withholding watches.
func (w *world) counting(prefix string) func() int {
	return func() int { return len(w.asked(prefix)) }
}
