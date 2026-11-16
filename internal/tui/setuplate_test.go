// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// setUpDialog is the first run's dialog, by its title.
const setUpDialog = "Set up workflow"

// jiraLeftOut answers each of the first run's forms handed to the prompts:
// the first place, and Jira's address left blank. A credential's prompt
// needs a terminal, so over a reader it is left blank too.
const jiraLeftOut = "1\n\n"

// promptedSetUp is a first run asking its forms as plain prompts, answered
// with the first place, Jira left out and no webhook. The prompts answer the
// first forms, as many as answeredAtOnce, as they are asked; the count grows
// as each later one runs, so withheldKeys can keep its answer back.
func promptedSetUp(t *testing.T, answeredAtOnce int32) (tui.Model, func() int) {
	t.Helper()

	prompts := &answering{typed: jiraLeftOut}

	var ran atomic.Int32

	deps := newFirstRun(t, http.StatusOK).deps(t)
	deps.Forms.Ask = func(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
		ask := prompts.ask(form, done)
		if prompts.asked.Load() <= answeredAtOnce {
			return ask
		}

		return func() tea.Msg {
			ran.Add(1)

			return ask()
		}
	}
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps).WithAccessibleForms(), 120, 40)

	return drain(t, model, model.Init()), func() int { return int(ran.Load()) }
}

func TestJirasFormAnsweredOnceItsDialogClosedMovesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		after   []string
		want    []string
		refused []string
	}{
		"its dialog closed": {
			after: []string{keyEsc}, want: []string{"enter sets one up here"}, refused: []string{setUpDialog},
		},
		"another dialog opened since": {
			after: []string{keyEsc, keyEnter}, want: []string{"Where should the file go?"},
			refused: []string{"Jira   (skipped)"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model, ran := promptedSetUp(t, 0)
			asked, late := withheldKeys(t, model, ran, keyEnter)
			moved, _ := withheldKeys(t, asked, ran, tt.after...)

			// Act
			view := handedOver(t, moved, late).View().Content

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refused...)
		})
	}
}

func TestTheRestsFormAnsweredOnceJirasIsAskedAgainMovesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	model, ran := promptedSetUp(t, 1)
	asked, late := withheldKeys(t, model, ran, keyEnter)
	back, _ := withheldKeys(t, asked, ran, keyEsc)

	// Act
	view := handedOver(t, back, late).View().Content

	// Assert
	requireScreen(t, view, "Jira's address")
	refuseScreen(t, view, "Write ")
}

func TestSetUpTakesNoKeyWhileItsFileIsWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	looking := typing(t, newFirstRun(t, http.StatusOK).model(t, false), throughEveryQuestion()...)
	writing, _ := pressed(t, looking, keyEnter)

	// Act
	after, cmd := pressed(t, writing, keyEsc)

	// Assert
	if cmd != nil {
		t.Error("esc while the file is written ran a command, want it ignored")
	}

	view := after.View().Content
	requireScreen(t, view, "writing")

	if footer := footerLine(view); !strings.Contains(footer, "ctrl+c") || strings.Contains(footer, "esc") {
		t.Errorf("footer %q, want only ctrl+c while the file is written", footer)
	}
}

func TestSetUpTakesNoPasteWhileJiraIsAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	run.held = newHold(t)
	keys := toTheToken()
	typed := typing(t, run.model(t, false), keys[:len(keys)-1]...)

	run.held.armed.Store(true)
	asking := holding(t, run.held, typed, keyEnter)

	// Act
	updated, cmd := asking.Update(tea.PasteMsg{Content: "x"})

	// Assert
	if cmd != nil {
		t.Error("a paste while Jira is asked ran a command, want it ignored")
	}

	if got, want := concrete(t, updated).View().Content, asking.View().Content; got != want {
		t.Errorf("a paste while Jira is asked changed the dialog:\n%s", plain(got))
	}
}
