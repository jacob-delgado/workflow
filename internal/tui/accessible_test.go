// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// answering is a Forms seam that answers a form's plain prompts with what is
// typed, as a user at the terminal would, and counts the forms it was handed.
type answering struct {
	typed string
	asked atomic.Int32
}

// ask runs the form accessibly over the typed answers.
func (a *answering) ask(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
	a.asked.Add(1)

	return func() tea.Msg {
		// One byte a read, as a terminal hands over what is typed: each prompt
		// reads its own line and leaves the rest for the next.
		typed := iotest.OneByteReader(strings.NewReader(a.typed))

		return done(form.WithAccessible(true).WithInput(typed).WithOutput(io.Discard).Run())
	}
}

func TestAccessibleFormsAskTheirFieldsAsPlainPrompts(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	prompts := &answering{typed: "2\nnil token\n"}
	deps := resolving.deps()
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(resolving.cfg, nil, deps).WithAccessibleForms(), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	done := typing(t, model, "t", keyEnter)

	// Assert
	if prompts.asked.Load() != 1 {
		t.Errorf("handed the prompts %d forms, want the one", prompts.asked.Load())
	}

	want := "transition PROJ-412 5 resolution=2 customfield_10200=nil token"
	if calls := resolving.asked("transition PROJ"); len(calls) != 1 || calls[0] != want {
		t.Errorf("transition calls = %q, want %q", calls, want)
	}

	requireScreen(t, done.View().Content, "● PROJ-412 is now Done")
}

func TestPromptsThatEndEarlySendNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	prompts := &answering{typed: "2\n"}
	deps := resolving.deps()
	deps.Forms.Ask = prompts.ask
	model := sized(t, tui.New(resolving.cfg, nil, deps).WithAccessibleForms(), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	stopped := typing(t, model, "t", keyEnter)

	// Assert
	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent a move whose prompts ended early: %q", calls)
	}

	requireScreen(t, stopped.View().Content, "✗")
}
