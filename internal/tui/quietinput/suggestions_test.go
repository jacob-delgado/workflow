// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// acceptingKeys is a key map whose next-field, tab, also accepts a
// completion, as the terminal's pull request composer keys its fields.
func acceptingKeys() *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	tab := key.NewBinding(key.WithKeys("tab"))
	keymap.Input = huh.InputKeyMap{AcceptSuggestion: tab, Next: tab, Prev: keymap.Input.Prev, Submit: tab}

	return keymap
}

// wordDeleting is the key the input under the field deletes a word on.
const wordDeleting = "ctrl+w"

// leavingOnWordDeleting is a key map whose next-field is wordDeleting, and
// whose completion is accepted on accept.
func leavingOnWordDeleting(accept string) *huh.KeyMap {
	keymap := huh.NewDefaultKeyMap()
	next := key.NewBinding(key.WithKeys(wordDeleting))
	keymap.Input = huh.InputKeyMap{
		AcceptSuggestion: key.NewBinding(key.WithKeys(accept)), Next: next, Prev: keymap.Input.Prev, Submit: next,
	}

	return keymap
}

func TestNextFieldTakesTheCompletionOnlyWhenItIsTheKeyThatAcceptsOne(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keymap *huh.KeyMap
		next   string
		want   string
	}{
		"the key that accepts a completion": {keymap: acceptingKeys(), next: "tab", want: baseBranch},
		"another key going on":              {keymap: defaultKeys(), next: "enter", want: "m"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var value string

			field := focused(quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch}), tt.keymap)
			typed(t, field, "m")

			// Act
			_, cmd := field.Update(pressed(tt.next))

			// Assert
			if !sameMsg(cmd, huh.NextField) || value != tt.want {
				t.Errorf("%s moved on %v leaving %q, want it to move on leaving %q",
					tt.next, sameMsg(cmd, huh.NextField), value, tt.want)
			}
		})
	}
}

// Only a completion is taken on the way out: the key leaving the field is
// never handed to the input under it, which would delete a word on this one.
func TestNextFieldNeverTypesItsOwnKeyIntoTheField(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		accept, typed string
	}{
		"a key that accepts no completion, with one suggested":   {accept: "ctrl+e", typed: "m"},
		"the key that accepts a completion, with none suggested": {accept: wordDeleting, typed: "x"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var value string

			field := quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch})
			typed(t, focused(field, leavingOnWordDeleting(tt.accept)), tt.typed)

			// Act
			_, cmd := field.Update(pressed(wordDeleting))

			// Assert
			if !sameMsg(cmd, huh.NextField) || value != tt.typed {
				t.Errorf("%s moved on %v leaving %q, want it to move on leaving %q as typed",
					wordDeleting, sameMsg(cmd, huh.NextField), value, tt.typed)
			}
		})
	}
}

func TestUpdateCompletesWhatIsTypedOnTheFormsKeyThatAcceptsACompletion(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch}), defaultKeys())
	typed(t, field, "m")

	// Act
	_, cmd := field.Update(pressed("ctrl+e"))

	// Assert
	if cmd != nil || value != baseBranch {
		t.Errorf("ctrl+e, the form's key accepting a completion, left %q with a command %v, want %q and none",
			value, cmd != nil, baseBranch)
	}
}

func TestNextFieldCompletesAPastedBeginning(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch}), acceptingKeys())
	field.Update(tea.PasteMsg{Content: "ma"})

	// Act
	field.Update(pressed("tab"))

	// Assert
	if value != baseBranch {
		t.Errorf("next-field after pasting ma left %q, want it completed to %q", value, baseBranch)
	}
}

func TestSuggestingNothingTakesBackWhatWasSuggested(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch}).Suggesting(nil)
	typed(t, focused(field, acceptingKeys()), "m")

	// Act
	_, cmd := field.Update(pressed("tab"))

	// Assert
	if !sameMsg(cmd, huh.NextField) || value != "m" {
		t.Errorf("next-field moved on %v and left %q once nothing was suggested, want it moved on with m, uncompleted",
			sameMsg(cmd, huh.NextField), value)
	}
}

// What the field holds before anything is typed in it, as the base a pull
// request's form starts from, is left as it is when the field is left, though
// a suggestion begins with it: only what is typed or pasted is completed.
func TestLeavingAFieldNothingWasTypedInLeavesItsValueUncompleted(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		offeredBefore [][]string
		pressedFirst  []string
	}{
		"left at once":                        {},
		"left after a key that types nothing": {pressedFirst: []string{"end"}},
		"left once suggestions were offered again": {
			offeredBefore: [][]string{{"release"}},
		},
		"left once suggestions were taken back and offered again": {
			offeredBefore: [][]string{{backportBranch}, nil},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			value := baseBranch
			field := quietinput.New(baseTitle, "", &value)

			for _, options := range tt.offeredBefore {
				field.Suggesting(options)
			}

			focused(field.Suggesting([]string{backportBranch, baseBranch}), acceptingKeys())

			for _, key := range tt.pressedFirst {
				field.Update(pressed(key))
			}

			// Act
			_, cmd := field.Update(pressed("tab"))

			// Assert
			if !sameMsg(cmd, huh.NextField) || value != baseBranch {
				t.Errorf("leaving the field moved on %v and left %q, want it moved on with %q as it was",
					sameMsg(cmd, huh.NextField), value, baseBranch)
			}
		})
	}
}

// Suggestions offered once something is typed complete it, as ones offered
// before it was typed do.
func TestSuggestionsOfferedOnceSomethingIsTypedCompleteIt(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value), acceptingKeys())
	typed(t, field, "m")
	field.Suggesting([]string{baseBranch})

	// Act
	field.Update(pressed("tab"))

	// Assert
	if value != baseBranch {
		t.Errorf("next-field after suggesting %q once m was typed left %q, want it completed to %q",
			baseBranch, value, baseBranch)
	}
}

// A paste into what was typed is matched again, as a typed key is, so the
// completion follows what the field now holds.
func TestAPasteAfterTypingCompletesToWhatTheFieldNowBegins(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value).Suggesting([]string{baseBranch, "master"}), acceptingKeys())
	typed(t, field, "m")
	field.Update(tea.PasteMsg{Content: "as"})

	// Act
	field.Update(pressed("tab"))

	// Assert
	if value != "master" {
		t.Errorf("next-field after typing m and pasting as left %q, want it completed to %q", value, "master")
	}
}

// What is typed whole as one of the suggestions is kept as typed, though a
// suggestion before it begins with it: typing main is asking for main.
func TestATypedSuggestionIsKeptThoughALongerOneBeginsWithIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		start                 string
		keysBefore, keysAfter []string
	}{
		"typed into an empty field":   {start: "", keysAfter: []string{"m", "a", "i", "n"}},
		"typed back over a base held": {start: baseBranch, keysAfter: []string{"backspace", "n"}},
		"typed before the suggestions were offered": {
			start: "", keysBefore: []string{"m", "a", "i", "n"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			value := tt.start
			field := focused(quietinput.New(baseTitle, "", &value), acceptingKeys())

			for _, key := range tt.keysBefore {
				field.Update(pressed(key))
			}

			field.Suggesting([]string{backportBranch, baseBranch})

			for _, key := range tt.keysAfter {
				field.Update(pressed(key))
			}

			// Act
			_, cmd := field.Update(pressed("tab"))

			// Assert
			if !sameMsg(cmd, huh.NextField) || value != baseBranch {
				t.Errorf("next-field after typing %q moved on %v and left %q, want it moved on with %q",
					baseBranch, sameMsg(cmd, huh.NextField), value, baseBranch)
			}
		})
	}
}
