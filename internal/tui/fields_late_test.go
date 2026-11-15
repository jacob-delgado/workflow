// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// promptedCounting is the world offering the resolve move with its fields,
// and the interface on it asking forms as plain prompts, answered with typed;
// the count grows as each command asking them runs, so withheldKeys can keep
// their answer back.
func promptedCounting(t *testing.T, typed string) (*world, tui.Model, func() int) {
	t.Helper()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	prompts := &answering{typed: typed}

	var ran atomic.Int32

	deps := resolving.deps()
	deps.Forms.Ask = func(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
		ask := prompts.ask(form, done)

		return func() tea.Msg {
			ran.Add(1)

			return ask()
		}
	}
	model := sized(t, tui.New(resolving.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	return resolving, drain(t, model, model.Init()), func() int { return int(ran.Load()) }
}

func TestFieldsFilledAfterTheirFormClosedSendNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		after []string
		want  []string
	}{
		"the form closed, its picker still open": {after: []string{keyEsc}, want: []string{resolveChosen}},
		"its picker closed":                      {after: []string{keyEsc, keyEsc}, want: []string{"1 Issues"}},
		"another form opened in its picker":      {after: []string{keyEsc, keyEnter}, want: []string{resolutionAsked}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			resolving, model, ran := promptedCounting(t, "2\nnil token\n")
			asked, late := withheldKeys(t, model, ran, "t", keyEnter)
			moved, _ := withheldKeys(t, asked, ran, tt.after...)

			// Act
			view := handedOver(t, moved, late).View().Content

			// Assert
			if calls := resolving.asked("transition "); len(calls) != 0 {
				t.Errorf("fields filled for a form since closed sent %q", calls)
			}

			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, "is now", "changing")
		})
	}
}

func TestFieldsFilledForAClosedPickerLeaveTheOneOpenedSinceFilling(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving, model, ran := promptedCounting(t, "2\nnil token\n")
	asked, late := withheldKeys(t, model, ran, "t", keyEnter)
	reopened, _ := withheldKeys(t, asked, ran, keyEsc, keyEsc, "t", keyEnter)

	// Act
	view := handedOver(t, reopened, late).View().Content

	// Assert
	if calls := resolving.asked("transition "); len(calls) != 0 {
		t.Errorf("fields filled for a closed picker sent %q from the one opened since", calls)
	}

	requireScreen(t, view, resolutionAsked)
	refuseScreen(t, view, "is now", "changing", "✗")
}

func TestAKeyAfterTheLastFieldIsAppliedChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	filled := typing(t, resolving.live(t, 120, 40), append([]string{"t", keyEnter, keyEnter}, letters("nil token")...)...)
	submitted, _ := pressed(t, filled, keyEnter)

	// Act
	after, cmd := pressed(t, submitted, keyEsc)

	// Assert
	if cmd != nil {
		t.Error("esc after the last field was applied produced a command")
	}

	if after.View().Content != submitted.View().Content {
		t.Errorf("esc after the last field was applied changed the screen:\n%s", after.View().Content)
	}
}
