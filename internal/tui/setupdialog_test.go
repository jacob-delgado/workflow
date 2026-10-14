// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// escBacks is a footer's esc where it goes back a step.
const escBacks = "esc back"

// errPromptsStopped is the prompts ending before every question was asked,
// as a terminal closed mid-form ends them.
var errPromptsStopped = errors.New("the prompts stopped")

func TestSetUpMasksACredentialAsItIsTyped(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys   []string
		secret string
	}{
		"the token": {
			keys:   []string{keyEnter, keyEnter, pasted(firstRunJira), keyEnter, pasted(firstRunToken)},
			secret: firstRunToken,
		},
		"the webhook": {
			keys:   append(toTheToken(), keyEnter, pasted(firstRunWebhook)),
			secret: firstRunWebhook,
		},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := typing(t, newFirstRun(t, http.StatusOK).model(t, false), each.keys...).View().Content

			// Assert
			requireScreen(t, view, strings.Repeat("*", len(each.secret)))
			refuseScreen(t, view, each.secret)
		})
	}
}

func TestSetUpFloatsAsADialogOverTheScreen(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newFirstRun(t, http.StatusOK).model(t, false), keyEnter).View().Content

	// Assert
	if _, column, found := cornerOf(view, "Set up workflow"); !found || column >= detailColumn {
		t.Errorf("the setup is not a dialog over the middle of the screen (column %d):\n%s", column, plain(view))
	}
}

func TestSetUpTurnsTheInFlightMarkWhileJiraIsAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	frames, run := &stepper{}, newFirstRun(t, http.StatusOK)
	run.held = newHold(t)
	deps := run.deps(t)
	deps.After = frames.after
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps), 120, 40)
	typed := typing(t, drain(t, model, model.Init()),
		keyEnter, keyEnter, pasted(firstRunJira), keyEnter, pasted(firstRunToken))

	run.held.armed.Store(true)
	asking := holding(t, run.held, typed, keyEnter)

	// Act
	turned := frames.step(t, asking)

	// Assert
	requireScreen(t, turned.View().Content, "◓ checking the token with Jira")
}

func TestSetUpHandsEachFormToThePromptsWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	// The same answers serve each form handed over: the first place, and
	// Jira's address left blank. A credential's prompt needs a terminal, so
	// over a reader it is left blank too.
	prompts := &answering{typed: "1\n\n"}
	deps := newFirstRun(t, http.StatusOK).deps(t)
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps).WithAccessibleForms(), 120, 40)

	// Act
	asked := typing(t, drain(t, model, model.Init()), keyEnter)

	// Assert
	if prompts.asked.Load() != 2 {
		t.Errorf("handed the prompts %d forms, want Jira's and the rest", prompts.asked.Load())
	}

	requireScreen(t, asked.View().Content, "Jira   (skipped)", "Slack  (skipped)", "Write ")
}

func TestSetUpKeepsEscInANarrowFooter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"on the first question": {keys: []string{keyEnter}, want: "esc cancel"},
		"on a later one":        {keys: []string{keyEnter, keyEnter}, want: escBacks},
		"on the last look":      {keys: throughEveryQuestion(), want: escBacks},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := sized(t, tui.New(config.Default(), config.ErrNotFound, newFirstRun(t, http.StatusOK).deps(t)), 80, 24)

			// Act
			view := typing(t, drain(t, model, model.Init()), each.keys...).View().Content

			// Assert
			if footer := footerLine(view); !strings.Contains(footer, each.want) {
				t.Errorf("the footer = %q, want %q", footer, each.want)
			}
		})
	}
}

func TestSetUpsLastLookGoesBackOnPreviousField(t *testing.T) {
	t.Parallel()

	// Arrange
	looking := typing(t, newFirstRun(t, http.StatusOK).model(t, false), throughEveryQuestion()...)

	// Act
	back := typing(t, looking, keyShiftTab)

	// Assert
	requireScreen(t, back.View().Content, askedWebhook)
}

func TestSetUpSaysWhyItsPromptsStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newFirstRun(t, http.StatusOK).deps(t)
	deps.Forms.Ask = func(_ *huh.Form, done func(error) tea.Msg) tea.Cmd {
		return func() tea.Msg { return done(errPromptsStopped) }
	}
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps).WithAccessibleForms(), 120, 40)

	// Act
	stopped := typing(t, drain(t, model, model.Init()), keyEnter)

	// Assert
	requireScreen(t, stopped.View().Content, "the prompts stopped", "Where should the file go?")
}

func TestSetUpTakesItsAnswersInTheDialogOnceItsPromptsStopped(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		// leftBehind is what the prompts leave the form submitting with when
		// they stop.
		leftBehind tea.Cmd
	}{
		"before they asked anything": {leftBehind: nil},
		"after Huh ran them":         {leftBehind: tea.Quit},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := newFirstRun(t, http.StatusOK).deps(t)
			deps.Forms.Ask = func(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
				form.SubmitCmd = each.leftBehind

				return func() tea.Msg { return done(errPromptsStopped) }
			}
			model := sized(t, tui.New(config.Default(), config.ErrNotFound, deps).WithAccessibleForms(), 120, 40)
			stopped := typing(t, drain(t, model, model.Init()), keyEnter)

			// Act
			answered := typing(t, stopped, keyEnter, keyEnter)

			// Assert
			requireScreen(t, answered.View().Content, "Jira   (skipped)", askedWebhook)
		})
	}
}

func TestSetUpsLastLookTakesNoPaste(t *testing.T) {
	t.Parallel()

	// Arrange
	looking := typing(t, newFirstRun(t, http.StatusOK).model(t, false), throughEveryQuestion()...)

	// Act
	pasted := pasting(t, looking, "x")

	// Assert
	if got, want := pasted.View().Content, looking.View().Content; got != want {
		t.Errorf("a paste changed the last look:\n%s", plain(got))
	}
}

func TestSetUpTakesNoKeyWhileJiraIsAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	run.held = newHold(t)
	keys := toTheToken()
	typed := typing(t, run.model(t, false), keys[:len(keys)-1]...)

	run.held.armed.Store(true)
	asking := holding(t, run.held, typed, keyEnter)

	// Act
	pressedEsc := holding(t, run.held, asking, keyEsc)

	// Assert
	requireScreen(t, pressedEsc.View().Content, "checking the token with Jira")
}

func TestSetUpFollowsTheTerminalWhenItIsResized(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	model := sized(t, tui.New(config.Default(), config.ErrNotFound, run.deps(t)), 120, 40)
	long := "https://jira.example.com/a/long/path/to/the/tracker/ending/here"
	asking := typing(t, drain(t, model, model.Init()), keyEnter, keyEnter, pasted(long))

	// Act
	narrowed, cmd := asking.Update(tea.WindowSizeMsg{Width: 70, Height: 40})
	narrow := drain(t, concrete(t, narrowed), cmd)

	// Assert
	// The field fits the narrower dialog: the address, which the wide one
	// showed whole, scrolls along its one line to the cursor at its end,
	// rather than running past the frame.
	if line := lineShowing(t, narrow.View().Content, "ending/here"); strings.Contains(line, "https:") {
		t.Errorf("the address's line is as wide as before the resize: %s", line)
	}
}
