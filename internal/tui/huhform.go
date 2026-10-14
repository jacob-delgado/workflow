// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// relaying is an overlay holding a Huh form, which the messages Update has no
// case of its own for are handed to: the form's own, on their way from one
// field to the next.
type relaying interface {
	relay(m Model, msg tea.Msg) (Model, tea.Cmd)
}

// huhTheme is Huh drawn the way the rest of the interface is: in the
// terminal's own palette, through kit's styles, with this session's glyphs
// for the cursor, the checkboxes and a refusal.
func huhTheme(kit renderKit) huh.ThemeFunc {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		theme := huh.ThemeBase(false)
		theme.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		theme.Focused = huhFieldStyles(kit, kit.marks.selected)
		theme.Blurred = huhFieldStyles(kit, kit.marks.unselected)
		theme.Help.ShortKey, theme.Help.ShortDesc = kit.styles.label, kit.styles.label
		theme.Help.ShortSeparator = kit.styles.label

		return theme
	})
}

// huhFieldStyles is how a field is drawn, its cursor row marked by selector:
// plain, its title strong, a hint faint, and a refusal red behind the failure
// mark.
func huhFieldStyles(kit renderKit, selector string) huh.FieldStyles {
	plain := lipgloss.NewStyle()

	var styles huh.FieldStyles

	styles.Title, styles.NoteTitle, styles.Description = kit.styles.strong, kit.styles.strong, kit.styles.label
	styles.ErrorIndicator = plain
	styles.ErrorMessage = kit.styles.failure.SetString(kit.marks.failed)
	styles.SelectSelector = plain.SetString(selector)
	styles.MultiSelectSelector = plain.SetString(selector)
	styles.SelectedPrefix = plain.SetString(checkbox(true))
	styles.UnselectedPrefix = plain.SetString(checkbox(false))
	styles.TextInput.Prompt = plain
	styles.TextInput.Placeholder = kit.styles.label
	styles.FocusedButton = kit.styles.strong.Reverse(true)
	styles.BlurredButton = plain

	return styles
}

// huhKeys is Huh's key map made from the interface's, so ui.keys moves a key
// inside a form as it does everywhere else: next-field, and apply on any
// field but the last, go on to the next; previous-field goes back; apply on
// the last field submits; up, down, first, last and toggle-option move and
// pick. Huh's own quit is off, since interrupt and close are answered before
// a form sees a key, and so are its filters, which would take a typed /.
func huhKeys(keys keyMap) *huh.KeyMap {
	next := key.NewBinding(key.WithKeys(slices.Concat(keys.confirm.Keys(), keys.nextField.Keys())...),
		key.WithHelp(keys.confirm.Help().Key, "next"))
	prev, submit := keys.prevField, relabel(keys.confirm, "apply")
	off := key.NewBinding(key.WithDisabled())

	keymap := huh.NewDefaultKeyMap()
	keymap.Quit = off
	keymap.Input = huh.InputKeyMap{AcceptSuggestion: off, Next: next, Prev: prev, Submit: submit}
	keymap.Select = huh.SelectKeyMap{
		Next: next, Prev: prev, Submit: submit, Up: keys.up, Down: keys.down, GotoTop: keys.first, GotoBottom: keys.last,
		HalfPageUp: off, HalfPageDown: off, Left: off, Right: off, Filter: off, SetFilter: off, ClearFilter: off,
	}
	keymap.MultiSelect = huh.MultiSelectKeyMap{
		Next: next, Prev: prev, Submit: submit, Up: keys.up, Down: keys.down, GotoTop: keys.first,
		GotoBottom: keys.last, Toggle: keys.toggleOption, HalfPageUp: off, HalfPageDown: off, Filter: off,
		SetFilter: off, ClearFilter: off, SelectAll: off, SelectNone: off,
	}

	return keymap
}

// newHuhForm is a form of groups drawn and keyed as the interface is, its help
// left to the footer, started, and sized to width columns and as many of rows
// as its tallest group needs; and the command that goes on starting it.
func newHuhForm(kit renderKit, keys keyMap, width, rows int, groups ...*huh.Group) (*huh.Form, tea.Cmd) {
	form := huh.NewForm(groups...).WithTheme(huhTheme(kit)).WithKeyMap(huhKeys(keys)).WithShowHelp(false)
	start := form.Init()
	// Sized as Huh sizes itself on the window it asks for once started: each
	// group as tall as the tallest needs, within rows.
	form.Update(tea.WindowSizeMsg{Width: width, Height: rows})

	return form, start
}

// formKeys is what a form's focused field answers now, for the footer.
func formKeys(form *huh.Form) []key.Binding {
	return slices.DeleteFunc(form.KeyBinds(), func(binding key.Binding) bool { return !binding.Enabled() })
}

// relayToForm hands msg to form, all but a cursor's blink: an embedded field
// never blinks, so no timer runs for the life of the form.
func relayToForm(form *huh.Form, msg tea.Msg) tea.Cmd {
	// Trade-off TRADE-46: Huh offers no way to stop a field's cursor blinking,
	// so its blink is dropped here, after the real wait each one costs.
	if _, blink := msg.(cursor.BlinkMsg); blink {
		return nil
	}

	_, cmd := form.Update(msg)

	return cmd
}

// pressOf is a press of the first key binding answers, as the terminal would
// send it, so the wheel can move a form as its bound key does wherever
// ui.keys has moved it. A key it cannot name is a press nothing answers.
func pressOf(binding key.Binding) tea.KeyPressMsg {
	keys := binding.Keys()
	if len(keys) == 0 {
		return tea.KeyPressMsg{}
	}

	parts := strings.Split(keys[0], "+")
	press := tea.KeyPressMsg{Code: namedKey(parts[len(parts)-1])}

	for _, modifier := range parts[:len(parts)-1] {
		press.Mod |= map[string]tea.KeyMod{"ctrl": tea.ModCtrl, "alt": tea.ModAlt, "shift": tea.ModShift}[modifier]
	}

	return press
}

// namedKey is the key a name in a binding stands for: a named key, or the one
// character it is.
func namedKey(name string) rune {
	named := map[string]rune{
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"tab": tea.KeyTab, "enter": tea.KeyEnter, "esc": tea.KeyEscape, "space": tea.KeySpace,
	}
	if code, isNamed := named[name]; isNamed {
		return code
	}

	if runes := []rune(name); len(runes) == 1 {
		return runes[0]
	}

	return 0
}
