// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// resolveIssue is a transition behind a screen that asks for a resolution and a
// root cause.
func resolveIssue() jira.Transition {
	return jira.Transition{
		ID: "5", Name: "Resolve Issue", ToStatus: "Done", ToStatusCategory: "done",
		Fields: []jira.Field{
			{ID: "resolution", Name: "Resolution", Kind: jira.FieldOption, Options: []jira.Option{
				{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"},
			}},
			{ID: "customfield_10200", Name: "Root cause", Kind: jira.FieldText, Options: nil},
		},
	}
}

func TestATransitionNeedingFieldsAsksForEachInTurn(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	model := resolving.live(t, 120, 40)

	// Act: open the picker
	listed := typing(t, model, "t")

	// Assert: the transition says what it needs
	requireScreen(t, listed.View(), "Resolve Issue → Done · needs Resolution, Root cause")

	// Act: choose it
	resolution := typing(t, listed, keyEnter)

	// Assert: the first field offers its values
	requireScreen(t, resolution.View(), "Resolve Issue → Done needs:", "Resolution (1 of 2)", "▸ Fixed", "Won't Fix")

	// Act: pick the second value
	cause := typing(t, resolution, "j", keyEnter)

	// Assert: the text field is next, and nothing is sent until it is filled
	requireScreen(t, cause.View(), "Root cause (2 of 2)", "> ")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Fatalf("sent before every field was filled: %q", calls)
	}

	// Act: fill it in
	done := typing(t, cause, append(letters("nil token"), keyEnter)...)

	// Assert: the move is sent once, with both values
	requireScreen(t, done.View(), "● PROJ-412 is now Done")

	want := "transition PROJ-412 5 resolution=2 customfield_10200=nil token"
	if calls := resolving.asked("transition PROJ"); len(calls) != 1 || calls[0] != want {
		t.Errorf("transition calls = %q, want %q", calls, want)
	}
}

func TestATextFieldLeftEmptyIsNotSent(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	model := resolving.live(t, 120, 40)

	// Act: confirm the text field with only a space in it
	empty := typing(t, model, "t", keyEnter, keyEnter, " ", keyEnter)

	// Assert: it says a value is needed, and sends nothing
	requireScreen(t, empty.View(), "✗ Root cause needs a value", "Root cause (2 of 2)")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent with an empty field: %q", calls)
	}

	// Act: type something
	typed := typing(t, empty, "x")

	// Assert: the problem clears
	refuseScreen(t, typed.View(), "needs a value")
}

func TestWhereTheFieldsLeaveThePicker(t *testing.T) {
	t.Parallel()

	blocked := resolveIssue()
	blocked.Fields = append(blocked.Fields, jira.Field{ID: "assignee", Name: "Assignee", Kind: jira.FieldUnsupported})

	cases := map[string]struct {
		moves         []jira.Transition
		transitionErr error
		keys          []string
		want          []string
		refuse        []string
	}{
		"escape goes back to the transitions": {
			moves: []jira.Transition{resolveIssue()}, keys: []string{"t", keyEnter, "k", keyEsc},
			want: []string{"┏━ Change status", "▸ ● Resolve Issue"}, refuse: []string{"Resolution (1 of 2)"},
		},
		"a field only Jira can fill stops the transition here": {
			moves: []jira.Transition{blocked}, keys: []string{"t", keyEnter},
			want:   []string{"✗ Resolve Issue needs Assignee, which only Jira's own screen can fill"},
			refuse: []string{"Resolution (1 of"},
		},
		"a refusal after the fields keeps the picker open": {
			moves: []jira.Transition{resolveIssue()}, transitionErr: errNotVisible,
			keys: []string{"t", keyEnter, keyEnter, "x", keyEnter},
			want: []string{"┏━ Change status", "▸ ● Resolve Issue", "✗ jira rejected the request"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			resolving := newWorld()
			resolving.moves, resolving.transitionErr = tt.moves, tt.transitionErr

			// Act
			view := typing(t, resolving.live(t, 120, 40), tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want...)
			refuseScreen(t, view, tt.refuse...)
		})
	}
}

func TestTheFieldFootersOfferWhatWorks(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	model := resolving.live(t, 120, 40)

	// Act: open the field with a set of values
	options := typing(t, model, "t", keyEnter)

	// Assert: the list keys are offered
	requireScreen(t, footerLine(options.View()), "↑/k up", "enter apply")

	// Act: go on to the text field
	text := typing(t, options, keyEnter)

	// Assert: only what works in text is offered
	requireScreen(t, footerLine(text.View()), "enter apply", "esc close")
	refuseScreen(t, footerLine(text.View()), "↑/k")
}

func TestADryRunTransitionSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.moves = workflowMoves()

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	moved := typing(t, model, "t", "j", keyEnter)

	// Assert
	requireScreen(t, moved.View(), "dry run: would change PROJ-412 to Done")

	if calls := dry.asked("transition "); len(calls) != 0 {
		t.Errorf("a dry run transitioned: %q", calls)
	}
}
