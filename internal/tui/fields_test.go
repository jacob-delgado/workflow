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

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	listed := typing(t, resolving.live(t, 120, 40), "t")
	requireScreen(t, listed.View(), "Resolve Issue → Done · needs Resolution, Root cause")

	resolution := typing(t, listed, keyEnter)
	requireScreen(t, resolution.View(), "Resolve Issue → Done needs:", "Resolution (1 of 2)", "▸ Fixed", "Won't Fix")

	cause := typing(t, resolution, "j", keyEnter)
	requireScreen(t, cause.View(), "Root cause (2 of 2)", "> ")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Fatalf("sent before every field was filled: %q", calls)
	}

	done := typing(t, cause, append(letters("nil token"), keyEnter)...)
	requireScreen(t, done.View(), "● PROJ-412 moved to Done")

	want := "transition PROJ-412 5 resolution=2 customfield_10200=nil token"
	if calls := resolving.asked("transition PROJ"); len(calls) != 1 || calls[0] != want {
		t.Errorf("transition calls = %q, want %q", calls, want)
	}
}

func TestATextFieldLeftEmptyIsNotSent(t *testing.T) {
	t.Parallel()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	empty := typing(t, resolving.live(t, 120, 40), "t", keyEnter, keyEnter, " ", keyEnter)
	requireScreen(t, empty.View(), "✗ Root cause needs a value", "Root cause (2 of 2)")

	// Typing clears the problem.
	refuseScreen(t, typing(t, empty, "x").View(), "needs a value")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent with an empty field: %q", calls)
	}
}

func TestEscapeFromTheFieldsGoesBackToTheTransitions(t *testing.T) {
	t.Parallel()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	back := typing(t, resolving.live(t, 120, 40), "t", keyEnter, "k", "esc")
	requireScreen(t, back.View(), "┏━ Change status", "▸ ● Resolve Issue")
	refuseScreen(t, back.View(), "Resolution (1 of 2)")
}

func TestAFieldOnlyJiraCanFillStopsTheTransitionHere(t *testing.T) {
	t.Parallel()

	blocked := resolveIssue()
	blocked.Fields = append(blocked.Fields, jira.Field{ID: "assignee", Name: "Assignee", Kind: jira.FieldUnsupported})

	resolving := newWorld()
	resolving.moves = []jira.Transition{blocked}

	view := typing(t, resolving.live(t, 120, 40), "t", keyEnter).View()
	requireScreen(t, view, "✗ Resolve Issue needs Assignee, which only Jira's own screen can fill")
	refuseScreen(t, view, "Resolution (1 of")
}

func TestARefusalAfterTheFieldsKeepsThePickerOpen(t *testing.T) {
	t.Parallel()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	resolving.transitionErr = errNotVisible

	refused := typing(t, resolving.live(t, 120, 40), append([]string{"t", keyEnter, keyEnter},
		append(letters("x"), keyEnter)...)...)

	requireScreen(t, refused.View(), "┏━ Change status", "▸ ● Resolve Issue", "✗ jira rejected the request")
}

func TestTheFieldFootersOfferWhatWorks(t *testing.T) {
	t.Parallel()

	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	options := typing(t, resolving.live(t, 120, 40), "t", keyEnter)
	requireScreen(t, footerLine(options.View()), "↑/k up", "enter apply")

	text := typing(t, options, keyEnter)
	requireScreen(t, footerLine(text.View()), "enter apply", "esc close")
	refuseScreen(t, footerLine(text.View()), "↑/k")
}

func TestADryRunTransitionSaysWhatItWouldDo(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.moves = workflowMoves()

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	moved := typing(t, model, "t", "j", keyEnter)
	requireScreen(t, moved.View(), "dry run: would move PROJ-412 to Done")

	if calls := dry.asked("transition "); len(calls) != 0 {
		t.Errorf("a dry run transitioned: %q", calls)
	}
}
