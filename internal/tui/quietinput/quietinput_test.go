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
