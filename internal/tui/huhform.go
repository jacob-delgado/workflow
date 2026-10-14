// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"io"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// FormDeps hands a form the terminal to ask its questions as plain prompts,
// one at a time, which a screen reader reads as it would any command's. It
// takes the terminal while it asks, as the editor does.
type FormDeps struct {
	// Ask runs form's prompts and delivers done's message, with the error that
	// stopped it if any, once it has its answers. Nil hands the program's
	// terminal to Huh's accessible prompts through tea.Exec.
	Ask func(form *huh.Form, done func(error) tea.Msg) tea.Cmd
}

// ask hands form the terminal for its prompts, through the seam if one is
// given.
func (d Deps) ask(form *huh.Form, done func(error) tea.Msg) tea.Cmd {
	if d.Forms.Ask == nil {
		return tea.Exec(promptedForm{form: form}, done)
	}

	return d.Forms.Ask(form, done)
}

// promptedForm is a form run as Huh's accessible prompts on the terminal
// tea.Exec hands it.
type promptedForm struct{ form *huh.Form }

var _ tea.ExecCommand = promptedForm{}

// Run asks the form's questions one at a time.
func (p promptedForm) Run() error {
	// Trade-off TRADE-47: the form takes the whole terminal while it asks.
	return p.form.WithAccessible(true).Run() //nolint:wrapcheck // Huh's own words for a prompt it could not ask
}

// SetStdin is where the answers are read.
func (p promptedForm) SetStdin(in io.Reader) { p.form.WithInput(in) }

// SetStdout is where the questions are asked.
func (p promptedForm) SetStdout(out io.Writer) { p.form.WithOutput(out) }

// SetStderr is where nothing is written: Huh asks on its output alone.
func (promptedForm) SetStderr(io.Writer) {}

// relaying is an overlay holding a Huh form, which the messages Update has no
// case of its own for are handed to — the form's own, on their way from one
// field to the next — and which is fitted again to a resized screen.
type relaying interface {
	relay(m Model, msg tea.Msg) (Model, tea.Cmd)
	resized(m Model) (Model, tea.Cmd)
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

// refusalRows is what a Huh group draws under its field when it refuses an
// answer: a blank row and the reason.
const refusalRows = 2

// newHuhForm is a form of groups drawn and keyed as the interface is, its help
// left to the footer, started, and fitted to width columns and rows rows; and
// the command that goes on starting it.
func newHuhForm(kit renderKit, keys keyMap, width, rows int, groups ...*huh.Group) (*huh.Form, tea.Cmd) {
	form := huh.NewForm(groups...).WithTheme(huhTheme(kit)).WithKeyMap(huhKeys(keys)).WithShowHelp(false)
	start := form.Init()

	return form, tea.Batch(start, fitForm(form, width, rows))
}

// fitForm sizes form as Huh sizes itself on the window it is given: width
// columns, and each group as tall as the tallest needs within rows less a
// refusal's, which Huh draws under a group already as tall as it is sized.
func fitForm(form *huh.Form, width, rows int) tea.Cmd {
	return relayToForm(form, tea.WindowSizeMsg{Width: width, Height: max(1, rows-refusalRows)})
}

// startForm is the command that starts form: drawn in the overlay that holds
// it, or, with accessible forms on, handed to the prompts. Either way done's
// message arrives once it is answered, with the error that stopped the
// prompts if any.
func (m Model) startForm(form *huh.Form, start tea.Cmd, done func(error) tea.Msg) tea.Cmd {
	if m.accessibleForms {
		return m.deps.ask(form, done)
	}

	answerInPlace(form, done(nil))

	return start
}

// answerInPlace has form, drawn in the overlay that holds it, deliver
// answered once it is answered, rather than what prompts that stopped left it
// delivering: Huh's own run of them leaves it quitting the program.
func answerInPlace(form *huh.Form, answered tea.Msg) {
	form.SubmitCmd = func() tea.Msg { return answered }
	form.CancelCmd = nil
}

// formKeys is what a form's focused field answers now, for the footer.
func formKeys(form *huh.Form) []key.Binding {
	return slices.DeleteFunc(form.KeyBinds(), func(binding key.Binding) bool { return !binding.Enabled() })
}

// relayToForm hands msg to form, and passes on what it asks for.
func relayToForm(form *huh.Form, msg tea.Msg) tea.Cmd {
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
