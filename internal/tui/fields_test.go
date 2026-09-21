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

// fieldfulMove is a transition whose screen asks for a user, a date and a
// list of fix versions — one kind of each the form now fills.
func fieldfulMove() jira.Transition {
	return jira.Transition{
		ID: "9", Name: "Start", ToStatus: statusInProgress, ToStatusCategory: categoryIndeterminate,
		Fields: []jira.Field{
			{ID: idAssignee, Name: fieldAssignee, Kind: jira.FieldUser},
			{ID: "duedate", Name: "Due date", Kind: jira.FieldDate},
			{ID: "fixVersions", Name: "Fix Version/s", Kind: jira.FieldOptionList, Options: []jira.Option{
				{ID: "10000", Name: "1.0"}, {ID: "10001", Name: "1.1"},
			}},
		},
	}
}

func TestATransitionFillsAUserDateAndSeveralVersions(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}
	model := moving.live(t, 120, 40)

	// Act: open the picker and choose the move
	form := typing(t, model, "t", keyEnter)

	// Assert: the user field is first
	requireScreen(t, form.View().Content, "Assignee (1 of 3)")

	// Act: type a username
	afterUser := typing(t, form, append(letters("fred"), keyEnter)...)

	// Assert: the date field is next
	requireScreen(t, afterUser.View().Content, "Due date (2 of 3)")

	// Act: type a date
	afterDate := typing(t, afterUser, append(letters("2026-09-21"), keyEnter)...)

	// Assert: the versions are next, none chosen yet, and the toggle is offered
	requireScreen(t, afterDate.View().Content, "Fix Version/s (3 of 3)", "○ 1.0", "○ 1.1")
	requireScreen(t, footerLine(afterDate.View().Content), "space select")

	// Act: choose both and apply
	done := typing(t, afterDate, keySpace, "j", keySpace, keyEnter)

	// Assert: the move is sent once, with all three values in the shape each needs
	requireScreen(t, done.View().Content, "● PROJ-412 is now In Progress")

	want := "transition PROJ-412 9 assignee=fred duedate=2026-09-21 fixVersions=10000,10001"
	if calls := moving.asked("transition PROJ"); len(calls) != 1 || calls[0] != want {
		t.Errorf("transition calls = %q, want %q", calls, want)
	}
}

func TestADateFieldRefusesWhatIsNotADate(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}
	model := moving.live(t, 120, 40)

	// Act
	dated := typing(t, typing(t, model, "t", keyEnter), append(letters("fred"), keyEnter)...)
	refused := typing(t, dated, append(letters("soon"), keyEnter)...)

	// Assert
	requireScreen(t, refused.View().Content, "✗ Due date must be a date like 2026-09-21", "Due date (2 of 3)")

	if calls := moving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent with a bad date: %q", calls)
	}
}

func TestAListFieldNeedsAtLeastOneChoice(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}
	model := moving.live(t, 120, 40)

	// Act
	withUser := typing(t, typing(t, model, "t", keyEnter), append(letters("fred"), keyEnter)...)
	versions := typing(t, withUser, append(letters("2026-09-21"), keyEnter)...)
	empty := typing(t, versions, keyEnter)

	// Assert
	requireScreen(t, empty.View().Content, "✗ Fix Version/s needs at least one", "Fix Version/s (3 of 3)")

	if calls := moving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent with no version chosen: %q", calls)
	}
}

func TestAChosenVersionCanBeToggledOff(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}
	withUser := typing(t, typing(t, moving.live(t, 120, 40), "t", keyEnter), append(letters("fred"), keyEnter)...)
	versions := typing(t, withUser, append(letters("2026-09-21"), keyEnter)...)

	// Act: choose a version
	chosen := typing(t, versions, keySpace)

	// Assert: it shows as chosen
	requireScreen(t, chosen.View().Content, "● 1.0")

	// Act: choose it again
	off := typing(t, chosen, keySpace)

	// Assert: neither version is chosen now
	requireScreen(t, off.View().Content, "○ 1.0", "○ 1.1")
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
	requireScreen(t, listed.View().Content,
		"Resolve Issue → Done · needs Resolution, Root cause")

	// Act: choose it
	resolution := typing(t, listed, keyEnter)

	// Assert: the first field offers its values
	requireScreen(t, resolution.View().Content,
		"Resolve Issue → Done needs:", "Resolution (1 of 2)", "▸ Fixed", "Won't Fix")

	// Act: pick the second value
	cause := typing(t, resolution, "j", keyEnter)

	// Assert: the text field is next, and nothing is sent until it is filled
	requireScreen(t, cause.View().Content,
		"Root cause (2 of 2)", "> ")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Fatalf("sent before every field was filled: %q", calls)
	}

	// Act: fill it in
	done := typing(t, cause, append(letters("nil token"), keyEnter)...)

	// Assert: the move is sent once, with both values
	requireScreen(t, done.View().Content,
		"● PROJ-412 is now Done")

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
	requireScreen(t, empty.View().Content,
		"✗ Root cause needs a value", "Root cause (2 of 2)")

	if calls := resolving.asked("transition PROJ"); len(calls) != 0 {
		t.Errorf("sent with an empty field: %q", calls)
	}

	// Act: type something
	typed := typing(t, empty, "x")

	// Assert: the problem clears
	refuseScreen(t, typed.View().Content, "needs a value")
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
			view := typing(t, resolving.live(t, 120, 40), tt.keys...).View().Content

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
	requireScreen(t, footerLine(options.View().Content), "↑/k up", "enter next", "esc back")

	// Act: go on to the text field
	text := typing(t, options, keyEnter)

	// Assert: only what works in text is offered
	requireScreen(t, footerLine(text.View().Content), "enter apply", "esc back")
	refuseScreen(t, footerLine(text.View().Content), "↑/k")
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
	requireScreen(t, moved.View().Content,
		"dry run: would change PROJ-412 to Done")

	if calls := dry.asked("transition "); len(calls) != 0 {
		t.Errorf("a dry run transitioned: %q", calls)
	}
}

func TestTheFieldFormLabelsEscAndEnterForWhatTheyDo(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	// Act
	field := typing(t, resolving.live(t, 120, 40), "t", keyEnter)

	// Assert
	requireScreen(t, footerLine(field.View().Content), "enter next", "esc back")
}
