// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// The words the fields below are titled with and typed into.
const (
	baseTitle  = "Base"
	baseBranch = "main"
	// backportBranch is a branch whose name begins with baseBranch's.
	backportBranch = "main-backport"
)

// errRefused is what a field's check refuses with.
var errRefused = errors.New("refused")

// refusing refuses whatever it is asked to check.
func refusing(string) error { return errRefused }

// defaultKeys is Huh's own key map: ctrl+e accepts a completion, and enter
// and tab go on.
func defaultKeys() *huh.KeyMap {
	return huh.NewDefaultKeyMap()
}

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

// focused is field keyed by keymap and focused, as a form hands it the keys.
func focused(field *quietinput.Input, keymap *huh.KeyMap) *quietinput.Input {
	field.WithKeyMap(keymap)
	field.Focus()

	return field
}

// pressed is the press of the key named, as Bubble Tea delivers it.
func pressed(name string) tea.KeyPressMsg {
	switch name {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "ctrl+e":
		return tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
	case wordDeleting:
		return tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

// typed types text into field a key at a time.
func typed(t *testing.T, field *quietinput.Input, text string) {
	t.Helper()

	for _, letter := range text {
		if _, cmd := field.Update(pressed(string(letter))); cmd != nil {
			t.Fatalf("typing %q asked for a command, want none", letter)
		}
	}
}

// sameMsg reports whether cmd delivers the message want does.
func sameMsg(cmd, want tea.Cmd) bool {
	return cmd != nil && cmd() == want()
}

func TestUpdateLeavesTheFieldAsItWasForAMessageNeitherAKeyNorAPaste(t *testing.T) {
	t.Parallel()

	// Arrange
	value := baseBranch
	field := focused(quietinput.New(baseTitle, "", &value).CheckedBy(refusing), defaultKeys())
	field.Update(pressed("enter"))

	// Act
	model, cmd := field.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Assert
	if model != field || cmd != nil || value != baseBranch || field.GetValue() != baseBranch ||
		!errors.Is(field.Error(), errRefused) {
		t.Errorf("after a resize Update answered the field itself %v, the value %q (field %v), the refusal %v and "+
			"a command %v; want the field, %q, %v and none",
			model == field, value, field.GetValue(), field.Error(), cmd != nil, baseBranch, errRefused)
	}
}

func TestUpdateTakesBackARefusalOnceMoreIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	value := baseBranch
	field := focused(quietinput.New(baseTitle, "", &value).CheckedBy(refusing), defaultKeys())
	field.Update(pressed("enter"))

	// Act
	field.Update(pressed("x"))

	// Assert
	err := field.Error()
	if err != nil {
		t.Errorf("typing after a refusal left the refusal %v, want it taken back until the field is checked again", err)
	}
}

func TestUpdateTypesAKeyIntoTheFieldAndItsValue(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value), defaultKeys())

	// Act
	_, cmd := field.Update(pressed("x"))

	// Assert
	if cmd != nil || value != "x" || field.GetValue() != "x" {
		t.Errorf("typing x left the value %q (field %v) with a command %v, want x and none",
			value, field.GetValue(), cmd != nil)
	}
}

func TestUpdateTypesAPasteWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value), defaultKeys())

	// Act
	_, cmd := field.Update(tea.PasteMsg{Content: "origin/main"})

	// Assert
	if cmd != nil || value != "origin/main" {
		t.Errorf("pasting left the value %q with a command %v, want origin/main and none", value, cmd != nil)
	}
}

func TestUpdateMovesBackOnPreviousFieldWithoutCheckingWhatWasTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value).CheckedBy(refusing), defaultKeys())

	// Act
	_, cmd := field.Update(pressed("shift+tab"))

	// Assert
	if !sameMsg(cmd, huh.PrevField) || field.Error() != nil {
		t.Errorf("previous-field gave a command moving back %v and the refusal %v, want it to move back unchecked",
			sameMsg(cmd, huh.PrevField), field.Error())
	}
}

func TestUpdateMovesOnOnNextFieldOnlyOnceWhatWasTypedPassesItsCheck(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		check     func(string) error
		wantMoved bool
		wantErr   error
	}{
		"passing its check":    {check: func(string) error { return nil }, wantMoved: true, wantErr: nil},
		"refused by its check": {check: refusing, wantMoved: false, wantErr: errRefused},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			value := baseBranch
			field := focused(quietinput.New(baseTitle, "", &value).CheckedBy(tt.check), defaultKeys())

			// Act
			_, cmd := field.Update(pressed("enter"))

			// Assert
			if moved := sameMsg(cmd, huh.NextField); moved != tt.wantMoved || !errors.Is(field.Error(), tt.wantErr) {
				t.Errorf("next-field moved on %v with the refusal %v, want %v and %v",
					moved, field.Error(), tt.wantMoved, tt.wantErr)
			}
		})
	}
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

func TestInitStartsTheFieldUnfocused(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := focused(quietinput.New(baseTitle, "", &value), defaultKeys())

	// Act
	field.Init()

	// Assert
	field.Update(pressed("x"))

	if value != "" {
		t.Errorf("a key after Init typed %q, want nothing typed into an unfocused field", value)
	}
}

func TestBlurKeepsAndChecksWhatWasTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	var value, checked string

	field := focused(quietinput.New(baseTitle, "", &value).CheckedBy(func(text string) error {
		checked = text

		return errRefused
	}), defaultKeys())
	typed(t, field, baseBranch)
	// What the field writes to is set aside meanwhile, so only Blur's own
	// write keeps what was typed.
	value = ""

	// Act
	cmd := field.Blur()

	// Assert
	if cmd != nil || value != baseBranch || checked != baseBranch || !errors.Is(field.Error(), errRefused) {
		t.Errorf("leaving the field kept %q, checked %q, with the refusal %v and a command %v; want %q kept and "+
			"checked, %v and none", value, checked, field.Error(), cmd != nil, baseBranch, errRefused)
	}
}

func TestRunAccessibleAsksAsAPlainPromptAndKeepsTheAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	var (
		value string
		out   bytes.Buffer
	)

	field := quietinput.New(baseTitle, "", &value)

	// Act
	err := field.RunAccessible(&out, strings.NewReader(baseBranch+"\n"))

	// Assert
	if err != nil || value != baseBranch || !strings.Contains(out.String(), baseTitle) {
		t.Errorf("the prompt kept %q with %v, asking:\n%s\nwant %q kept under its title", value, err, out.String(),
			baseBranch)
	}
}

// Huh asks for a secret only through a terminal, which a plain reader is not,
// so a secret field refuses to be asked on one rather than echo the answer.
func TestRunAccessibleAsksForASecretOnlyOnATerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	var value string

	field := quietinput.New(baseTitle, "", &value).Secret()

	// Act
	err := field.RunAccessible(new(bytes.Buffer), strings.NewReader(baseBranch+"\n"))

	// Assert
	if err == nil || value != "" {
		t.Errorf("a secret asked on a plain reader kept %q with %v, want it refused and nothing kept", value, err)
	}
}

func TestTheFieldIsAskedAndHoldsItsValueRatherThanAKey(t *testing.T) {
	t.Parallel()

	// Arrange
	value := baseBranch
	field := quietinput.New(baseTitle, "", &value)

	// Act
	skipped, zoomed, named, held := field.Skip(), field.Zoom(), field.GetKey(), field.GetValue()

	// Assert
	if skipped || zoomed || named != "" || held != baseBranch {
		t.Errorf("Skip, Zoom, GetKey, GetValue = %v, %v, %q, %v, want neither skipped nor zoomed, no key, and %q",
			skipped, zoomed, named, held, baseBranch)
	}
}
