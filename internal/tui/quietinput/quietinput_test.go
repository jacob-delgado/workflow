// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package quietinput_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// The words the fields these tests ask are titled with and typed into.
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
