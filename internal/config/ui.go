// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
)

// ErrInvalidUI reports a ui setting the terminal interface could not honor.
var ErrInvalidUI = errors.New("invalid ui setting")

// The refusals of a ui.keys map the terminal interface cannot run, which its
// key check returns and every caller tells apart with errors.Is.
var (
	// ErrUnknownKeyAction reports a ui.keys entry naming an action that does not
	// exist — a typo in the action id.
	ErrUnknownKeyAction = errors.New("ui.keys names a key action that does not exist")
	// ErrKeyConflict reports two actions bound to the same key where both are
	// live at once, so a press would be ambiguous.
	ErrKeyConflict = errors.New("ui.keys binds two actions to one key in the same context")
	// ErrKeyNotRebindable reports a ui.keys entry moving an action whose keys
	// cannot be one key: jump-to-pane answers the pane numbers, one per pane.
	ErrKeyNotRebindable = errors.New("ui.keys moves an action whose keys cannot be rebound")
	// ErrInterruptEdits reports interrupt moved onto a key that types or edits
	// text. Interrupt is answered before any filter, prompt or text box, so on
	// such a key it would quit mid-sentence and lose what was written.
	ErrInterruptEdits = errors.New("ui.keys moves interrupt onto a key that types or edits text")
	// ErrTextFieldKey reports an overlay's key moved onto one its focused text
	// field edits or moves its cursor with — a character, an arrow, or a
	// readline key such as ctrl+w. The overlay answers the key first, so the
	// field would lose that edit.
	ErrTextFieldKey = errors.New("ui.keys moves an overlay's key onto one its text field edits with")
	// ErrKeyConfirmsItself reports an action that asks first — its press opens
	// a last look, which apply confirms — sharing a key with apply, so the key
	// pressed twice would ask and then confirm, as enter twice on Local data
	// would remove everything.
	ErrKeyConfirmsItself = errors.New("ui.keys puts an action that asks first on the key that confirms it")
)

// KeyRefusals are every refusal of a ui.keys map, so a caller sorting errors
// by kind counts a new one without a second edit.
func KeyRefusals() []error {
	return []error{
		ErrUnknownKeyAction, ErrKeyConflict, ErrKeyNotRebindable, ErrInterruptEdits, ErrTextFieldKey,
		ErrKeyConfirmsItself,
	}
}

// colorNever is the ui.color that turns the system hues off.
const colorNever = "never"

// MotionReduce is the ui.motion that draws every state still: no mark turns,
// no rule fills, no bar springs.
const MotionReduce = "reduce"

// UI is how the terminal interface behaves, and the one keyboard setting the
// --web page reads beside its keys.
type UI struct {
	// Mouse captures the mouse, so a click focuses a pane or selects a row.
	// Capturing it takes away the terminal's own click-and-drag selection,
	// which is why it can be turned off here, and toggled with m in a session.
	Mouse bool `json:"mouse"`
	// ASCII draws borders and glyphs in plain ASCII, for a terminal or font
	// without box-drawing characters. There is no reliable way to detect that,
	// so it is a setting rather than a guess.
	ASCII bool `json:"ascii"`
	// Color is whether to draw the system hues: empty (auto) draws them when the
	// terminal allows, "never" turns them off while keeping bold, faint and the
	// reverse-video cursor, which carry meaning without color.
	Color string `json:"color"`
	// Motion is whether the terminal interface moves: empty turns the in-flight
	// mark, fills a stage's rule as it completes and springs the CI bar;
	// "reduce" draws every state still, for anyone motion distracts or harms.
	Motion string `json:"motion"`
	// Accessible hands each multi-field form in the terminal the whole
	// terminal as a series of plain prompts, one question at a time, which a
	// screen reader reads as it would any command's, instead of drawing it
	// over the screen. The ACCESSIBLE environment variable, set to anything,
	// does the same.
	Accessible bool `json:"accessible"`
	// Notify rings the terminal, and sends a desktop notification where the
	// terminal relays one, when CI finishes — and the --web page says so in its
	// header — so a developer who stepped away is told rather than having to
	// check back. Off unless set.
	Notify bool `json:"notify"`
	// CommentsShown is how many of an issue's most recent comments the detail
	// pane draws. Zero keeps the built-in default.
	CommentsShown int `json:"comments_shown"`
	// Keys rebinds the interface's keys. Each entry maps an action to the single
	// key that should trigger it, e.g. {"commit": "C", "comment": "ctrl+e"}; the
	// help then shows the new key. An action left out keeps its default. The
	// interface refuses to start on a map that names an action it does not know,
	// moves jump-to-pane (its keys are the pane numbers, which no one key can
	// stand in for), or binds two actions in the same context to one key. The
	// actions it can rebind are listed under "Rebinding keys" in
	// docs/content/docs/configuration.md, which a terminal test holds to the
	// bound ones.
	Keys map[string]string `json:"keys"`
	// WebShortcuts turns on the --web page's single-key shortcuts — the
	// terminal's keys, ui.keys applied, pressed outside a text field. Off
	// unless set, since a character key that acts on its own surprises a
	// screen reader or speech user, who presses keys for other reasons (WCAG
	// 2.1.4). The ? sheet and the command palette, on ctrl+k or cmd+k, work
	// either way.
	WebShortcuts bool `json:"web_shortcuts"`
}

// DrawColor reports whether the system hues should be drawn. NO_COLOR set to
// a non-empty value and ui.color "never" both turn them off; bold, faint and
// reverse stay. An empty NO_COLOR keeps them, as no-color.org asks.
func (u UI) DrawColor(noColorEnv string) bool {
	return noColorEnv == "" && u.Color != colorNever
}

// AccessibleForms reports whether the terminal's forms ask as plain prompts:
// ui.accessible on, or ACCESSIBLE, whose value is accessibleEnv, set to
// anything at all.
func (u UI) AccessibleForms(accessibleEnv string) bool {
	return u.Accessible || accessibleEnv != ""
}

// ReducedMotion reports whether the interface draws every state still.
func (u UI) ReducedMotion() bool {
	return u.Motion == MotionReduce
}

// validateUI refuses a negative comments_shown — the detail pane keeps that many
// of an issue's most recent comments, and no count below zero names any — a
// color other than empty or never, which would draw the hues a misspelled never
// was meant to turn off, and a motion other than empty or reduce, which would
// move what a misspelled reduce was meant to still.
func (c Config) validateUI() error {
	if c.UI.CommentsShown < 0 {
		return fmt.Errorf("%w: comments_shown cannot be negative: %d", ErrInvalidUI, c.UI.CommentsShown)
	}

	if c.UI.Color != "" && c.UI.Color != colorNever {
		return fmt.Errorf("%w: color is empty or never: %q", ErrInvalidUI, c.UI.Color)
	}

	if c.UI.Motion != "" && c.UI.Motion != MotionReduce {
		return fmt.Errorf("%w: motion is empty or reduce: %q", ErrInvalidUI, c.UI.Motion)
	}

	return nil
}
