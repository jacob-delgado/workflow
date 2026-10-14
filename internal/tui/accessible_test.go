// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// answering is a Forms seam that answers a form's plain prompts with what is
// typed, as a user at the terminal would, and counts the forms it was handed
// and keeps what the prompts said.
type answering struct {
	typed string
	asked atomic.Int32
	// said is what the prompts wrote, read once the drain that ran them is
	// done.
	said strings.Builder
}

// ask runs the form accessibly over the typed answers.
func (a *answering) ask(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
	a.asked.Add(1)

	return func() tea.Msg {
		// One byte a read, as a terminal hands over what is typed: each prompt
		// reads its own line and leaves the rest for the next.
		typed := iotest.OneByteReader(strings.NewReader(a.typed))

		return done(form.WithAccessible(true).WithInput(typed).WithOutput(&a.said).Run())
	}
}

// promptedResolve is the world offering the resolve move with its fields, and
// the interface on it asking forms as plain prompts, answered with typed.
func promptedResolve(t *testing.T, typed string) (*world, *answering, tui.Model) {
	t.Helper()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	prompts := &answering{typed: typed}
	deps := resolving.deps()
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(resolving.cfg, nil, deps).WithAccessibleForms(), 120, 40)

	return resolving, prompts, drain(t, model, model.Init())
}

func TestAccessibleFormsAskTheirFieldsAsPlainPrompts(t *testing.T) {
	t.Parallel()

	// Arrange
	// A line typed past the last answer, as a terminal left open holds more
	// to read: each answer is taken as it is typed, not a later line.
	resolving, prompts, model := promptedResolve(t, "2\nnil token\nsecond try\n")

	// Act
	done := typing(t, model, "t", keyEnter)

	// Assert
	if prompts.asked.Load() != 1 {
		t.Errorf("handed the prompts %d forms, want the one", prompts.asked.Load())
	}

	if calls := resolving.asked("transition PROJ"); len(calls) != 1 || calls[0] != resolvedNilToken {
		t.Errorf("transition calls = %q, want %q", calls, resolvedNilToken)
	}

	if said := prompts.said.String(); strings.Contains(said, "needs a value") {
		t.Errorf("the prompts refused an answer typed in full:\n%s", said)
	}

	requireScreen(t, done.View().Content, "● PROJ-412 is now Done")
}

func TestPromptsThatEndEarlySendNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving, _, model := promptedResolve(t, "2\n")

	// Act
	stopped := typing(t, model, "t", keyEnter)

	// Assert
	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent a move whose prompts ended early: %q", calls)
	}

	requireScreen(t, stopped.View().Content, "✗")
}

func TestEachPlainPromptIsTitledAsTheFormTitlesItsFields(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"the resolution, chosen": "Resolution (1 of 2)",
		"the root cause, typed":  "Root cause (2 of 2)",
	}

	for name, title := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			_, prompts, model := promptedResolve(t, "2\nnil token\n")

			// Act
			typing(t, model, "t", keyEnter)

			// Assert
			if strong := opening(lipgloss.NewStyle().Bold(true)) + title; !strings.Contains(prompts.said.String(), strong) {
				t.Errorf("the prompt for %q is not titled in the form's strong style:\n%q", title, prompts.said.String())
			}
		})
	}
}

// The plain prompts a form is handed to ask a masked answer only on a
// terminal, which stops echoing while it is typed; over a plain reader, which
// could echo nothing away, they refuse it.
func TestAnAccessiblePasswordPromptOverAReaderIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	var secret string

	var said strings.Builder

	prompt := huh.NewInput().Title("Token").EchoMode(huh.EchoModePassword).Value(&secret)

	// Act
	err := prompt.RunAccessible(&said, strings.NewReader("hunter2\n"))

	// Assert
	if err == nil || secret != "" {
		t.Errorf("a masked prompt over a reader = %v, holding %q; want it refused, holding nothing", err, secret)
	}

	if strings.Contains(said.String(), "hunter2") {
		t.Errorf("a masked prompt over a reader echoed the answer:\n%s", said.String())
	}
}
