// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The refusals CheckKeys makes beyond two actions sharing a key where both
// are live: an action it does not know, one no single key can stand for, and
// a key the interface would read as something else first.

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// An action whose press opens a last look must not share a key with apply,
// which confirms the look: pressed twice, the key would ask and then confirm,
// and enter twice on Local data would remove everything.
func TestCheckKeysRefusesAnActionThatAsksFirstOnTheKeyThatConfirmsIt(t *testing.T) {
	t.Parallel()

	cases := map[string]map[string]string{
		"remove-everything onto apply's key": {"remove-everything": keyEnter},
		"remove-cache onto apply's key":      {"remove-cache": keyEnter},
		"apply onto remove-everything's key": {applyAction: "C"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, config.ErrKeyConfirmsItself) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrKeyConfirmsItself", rebound, err)
			}

			if got := err.Error(); !strings.Contains(got, "remove-") {
				t.Errorf("CheckKeys error %q does not name the action that asks first", got)
			}
		})
	}
}

func TestCheckKeysRefusesAKeyATextFieldEditsWithWhereOneHasTheFocus(t *testing.T) {
	t.Parallel()

	// Each action is answered by an overlay before its focused text field sees
	// the key, so on a key the field edits with it would take that edit away.
	cases := map[string]map[string]string{
		"worktree onto delete a word":            {worktreeAction: "ctrl+w"},
		"breaking onto back a character":         {"toggle-breaking": "ctrl+b"},
		"draft onto the line's start":            {toggleDraftAction: "ctrl+a"},
		"the next template onto next suggestion": {"next-template": "ctrl+n"},
		"edit body onto previous suggestion":     {editBodyAction: "ctrl+p"},
		"worktree onto a letter":                 {worktreeAction: "s"},
		"worktree onto the left arrow":           {worktreeAction: keyLeft},
		"draft onto the right arrow":             {toggleDraftAction: keyRight},
		"draft onto home":                        {toggleDraftAction: keyHome},
		"edit body onto end":                     {editBodyAction: keyEnd},
		"breaking onto the down arrow":           {"toggle-breaking": keyDown},
		"the next template onto the up arrow":    {"next-template": keyUp},
		"previous field onto back a word":        {"previous-field": "ctrl+left"},
		"next field onto delete a word forward":  {"next-field": "alt+d"},
	}

	for name, rebound := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tui.CheckKeys(rebound)

			// Assert
			if !errors.Is(err, config.ErrTextFieldKey) {
				t.Fatalf("CheckKeys(%v) = %v, want ErrTextFieldKey", rebound, err)
			}

			for action := range rebound {
				if !strings.Contains(err.Error(), action) {
					t.Errorf("CheckKeys error %q does not name %s", err, action)
				}
			}
		})
	}
}

func TestCheckKeysRefusesAnUnknownAction(t *testing.T) {
	t.Parallel()

	// Arrange
	misnamed := map[string]string{"no-such-action": "C"}

	// Act
	err := tui.CheckKeys(misnamed)

	// Assert
	if !errors.Is(err, config.ErrUnknownKeyAction) {
		t.Fatalf("CheckKeys = %v, want ErrUnknownKeyAction", err)
	}

	if got := err.Error(); !strings.Contains(got, "no-such-action") {
		t.Errorf("CheckKeys error %q does not name the unknown action", got)
	}
}

func TestCheckKeysRefusesMovingJumpToPane(t *testing.T) {
	t.Parallel()

	// Arrange
	// jump-to-pane answers one digit per pane, which no single key can stand
	// in for, so moving it to a free key is refused rather than accepted.
	moved := map[string]string{"jump-to-pane": "f12"}

	// Act
	err := tui.CheckKeys(moved)

	// Assert
	if !errors.Is(err, config.ErrKeyNotRebindable) {
		t.Fatalf("CheckKeys = %v, want ErrKeyNotRebindable", err)
	}

	if got := err.Error(); !strings.Contains(got, "jump-to-pane") {
		t.Errorf("CheckKeys error %q does not name the action", got)
	}
}

func TestCheckKeysRefusesMovingInterruptOntoAKeyThatTypesOrEdits(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a letter": "x", "a digit": "5", "a space": "space", "a symbol": "/",
		"a letter with an accent": "é", "an emoji with its variation selector": "\u2764\ufe0f",
		"backspace, which edits text": "backspace", "delete, which edits text": "delete",
	}

	for name, typed := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			// Interrupt is answered before any filter, prompt or text box, so on a
			// key that types it would quit mid-sentence and lose what was written.
			err := tui.CheckKeys(map[string]string{"interrupt": typed})

			// Assert
			if !errors.Is(err, config.ErrInterruptEdits) {
				t.Fatalf("CheckKeys = %v, want ErrInterruptEdits", err)
			}

			if got := err.Error(); !strings.Contains(got, "interrupt") {
				t.Errorf("CheckKeys error %q does not name the action", got)
			}
		})
	}
}

func TestCheckKeysAcceptsInterruptOnAKeyThatTypesNothing(t *testing.T) {
	t.Parallel()

	// Act
	err := tui.CheckKeys(map[string]string{"interrupt": "ctrl+q"})
	// Assert
	if err != nil {
		t.Errorf("CheckKeys = %v, want a control key accepted for interrupt", err)
	}
}
