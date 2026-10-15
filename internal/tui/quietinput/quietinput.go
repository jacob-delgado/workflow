// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package quietinput is a Huh field typing one line of text, as huh.Input
// does, over an input whose cursor never blinks: what every form of the
// terminal's takes typed text through.
package quietinput

import (
	"io"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Input is a Huh field typing one line of text, as huh.Input does, over an
// input whose cursor never blinks.
//
// Trade-off TRADE-46: huh.Input blinks its cursor and offers no way not to;
// each focus and each key schedules a blink that waits half a second of real
// time, which a test settling after every key pays in full.
type Input struct {
	title, hint, placeholder string
	value                    *string
	validate                 func(string) error
	input                    textinput.Model
	err                      error
	focused                  bool
	// options are the suggestions in the order they were offered.
	options []string
	// typedIn is whether a key or a paste has changed what the field holds,
	// before which nothing is completed.
	typedIn bool
	keymap  huh.InputKeyMap
	theme   huh.Theme
	width   int
}

var _ huh.Field = (*Input)(nil)

// New is a field titled title, with placeholder in it until something is
// typed, holding what value holds and writing what is typed back to it.
func New(title, placeholder string, value *string) *Input {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = placeholder
	input.SetValue(*value)

	return &Input{
		title: title, placeholder: placeholder, value: value, input: input,
		validate: func(string) error { return nil },
	}
}

// Init starts the field unfocused, as huh.Input does.
func (q *Input) Init() tea.Cmd {
	q.input.Blur()

	return nil
}

// Update answers a key or a paste: next-field, previous-field and submit move
// on once what was typed passes its check, and anything else is typed. What
// the input asks for in return — its blink, or a read of the clipboard — is
// never handed on, as the interface's other inputs never hand it on.
//
//nolint:ireturn // huh.Field's Update returns Huh's Model
func (q *Input) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	press, isKey := msg.(tea.KeyPressMsg)
	if _, isPaste := msg.(tea.PasteMsg); !isKey && !isPaste {
		return q, nil
	}

	q.err = nil

	switch {
	case isKey && key.Matches(press, q.keymap.Prev):
		return q, huh.PrevField
	case isKey && key.Matches(press, q.keymap.Next, q.keymap.Submit):
		q.completed(press)

		q.err = q.validate(q.input.Value())
		if q.err != nil {
			return q, nil
		}

		return q, huh.NextField
	}

	before := q.input.Value()
	q.input, _ = q.input.Update(msg)
	*q.value = q.input.Value()

	if *q.value != before {
		q.matchSuggestions()
	}

	return q, nil
}

// View is the field's title over its hint, if it has one, and the line
// typed, in the theme's styles for a field focused or not.
func (q *Input) View() string {
	styles := q.styles()

	lines := []string{styles.Title.Render(q.title)}
	if q.hint != "" {
		lines = append(lines, styles.Description.Render(q.hint))
	}

	return styles.Base.Width(q.width).Render(strings.Join(append(lines, q.input.View()), "\n"))
}

// Blur leaves the field, keeping and checking what was typed.
func (q *Input) Blur() tea.Cmd {
	q.focused = false
	q.input.Blur()
	*q.value = q.input.Value()
	q.err = q.validate(*q.value)

	return nil
}

// Focus enters the field, dropping the blink the input asks for.
func (q *Input) Focus() tea.Cmd {
	q.focused = true
	q.input.Focus()

	return nil
}

// Error is why what was typed was refused, if it was.
func (q *Input) Error() error {
	return q.err
}

// Run asks the field on its own, as Huh's input.
func (q *Input) Run() error {
	return q.accessible().Run() //nolint:wrapcheck // Huh's own words for a field it could not ask
}

// RunAccessible asks the field as a plain prompt on in and out.
func (q *Input) RunAccessible(out io.Writer, in io.Reader) error {
	return q.accessible().RunAccessible(out, in) //nolint:wrapcheck // as Run
}

// Skip reports that the field is asked, never passed over.
func (q *Input) Skip() bool { return false }

// Zoom reports that the field shares its group's height.
func (q *Input) Zoom() bool { return false }

// KeyBinds are the keys the field answers, for the footer.
func (q *Input) KeyBinds() []key.Binding {
	return []key.Binding{q.keymap.Prev, q.keymap.Submit, q.keymap.Next}
}

// WithTheme draws the field, and the text typed in it, in theme: the form's,
// since the field is given no theme of its own.
//
//nolint:ireturn // huh.Field's own signature
func (q *Input) WithTheme(theme huh.Theme) huh.Field {
	q.theme = theme
	q.input.SetStyles(textStyles(q.input.Styles(), theme.Theme(false)))

	return q
}

// WithKeyMap keys the field by keymap's input keys.
//
//nolint:ireturn // huh.Field's own signature
func (q *Input) WithKeyMap(keymap *huh.KeyMap) huh.Field {
	q.keymap = keymap.Input
	q.input.KeyMap.AcceptSuggestion = keymap.Input.AcceptSuggestion

	return q
}

// WithWidth fits the field to width columns, and its line to what is left
// of them once the prompt and the cursor at its end are drawn, scrolled so
// the cursor stays in it.
//
//nolint:ireturn // huh.Field's own signature
func (q *Input) WithWidth(width int) huh.Field {
	q.width = width
	frame := q.styles().Base.GetHorizontalFrameSize()
	q.input.SetWidth(max(1, width-frame-lipgloss.Width(q.input.Prompt)-1))
	// The input scrolls what is typed into its width only as the cursor
	// moves, so the cursor is set where it is to scroll it into the new one.
	q.input.SetCursor(q.input.Position())

	return q
}

// WithHeight leaves the field as tall as its title and its line: the group
// shows as much of it as fits.
//
//nolint:ireturn // huh.Field's own signature
func (q *Input) WithHeight(int) huh.Field {
	return q
}

// WithPosition offers previous-field everywhere but the first field, and
// next-field everywhere but the last, which submit answers instead.
//
//nolint:ireturn // huh.Field's own signature
func (q *Input) WithPosition(position huh.FieldPosition) huh.Field {
	q.keymap.Prev.SetEnabled(!position.IsFirst())
	q.keymap.Next.SetEnabled(!position.IsLast())
	q.keymap.Submit.SetEnabled(position.IsLast())

	return q
}

// GetKey is the field's key in the form's results: it has none, since what
// it holds is written to its value.
func (q *Input) GetKey() string { return "" }

// GetValue is what was typed.
func (q *Input) GetValue() any { return q.input.Value() }

// Suggesting is the field completing what is typed to the first of options
// it begins, or keeping it when it is one of them, the completion shown faint
// after it until accepted. What the field held before anything was typed in
// it is left as it is: the base a pull request's form starts from is never
// completed to a newer branch it begins.
func (q *Input) Suggesting(options []string) *Input {
	q.options = options
	q.input.ShowSuggestions = q.typedIn
	q.input.SetSuggestions(q.ordered())

	return q
}

// Hinted is the field with hint under its title, saying what it is for.
func (q *Input) Hinted(hint string) *Input {
	q.hint = hint

	return q
}

// Secret is the field drawing a mask in place of what is typed, and asking
// as prompts without echoing it.
func (q *Input) Secret() *Input {
	q.input.EchoMode = textinput.EchoPassword

	return q
}

// CheckedBy is the field refusing, with check's reason, what check refuses.
func (q *Input) CheckedBy(check func(string) error) *Input {
	q.validate = check

	return q
}

// matchSuggestions matches what the field holds against the suggestions, once
// a key, a paste or a completion taken has changed it. The input matches only
// while its suggestions are on, so they are turned on with the first change,
// and it matches by itself after a key, not a paste, so what the field holds is
// matched here either way, and with the option it holds whole listed first.
func (q *Input) matchSuggestions() {
	q.typedIn = true
	q.input.ShowSuggestions = true
	q.input.SetSuggestions(q.ordered())
}

// ordered are the options as offered, but for one that is what the field
// holds whole, which comes first: the input completes to the first option
// that begins with what is typed, so main typed is kept as main, though
// main-backport, offered before it, begins with it too.
func (q *Input) ordered() []string {
	at := slices.Index(q.options, q.input.Value())
	if at < 0 {
		return q.options
	}

	return slices.Concat(q.options[at:at+1], q.options[:at], q.options[at+1:])
}

// completed takes the completion the field suggests for what is typed, when
// press is the key that takes one, before the field is left.
func (q *Input) completed(press tea.KeyPressMsg) {
	if q.input.CurrentSuggestion() == "" || !key.Matches(press, q.keymap.AcceptSuggestion) {
		return
	}

	q.input, _ = q.input.Update(press)
	*q.value = q.input.Value()

	q.matchSuggestions()
}

// styles are the theme's styles for the field as it is now.
func (q *Input) styles() huh.FieldStyles {
	theme := q.theme
	if theme == nil {
		theme = huh.ThemeFunc(huh.ThemeBase)
	}

	if q.focused {
		return theme.Theme(false).Focused
	}

	return theme.Theme(false).Blurred
}

// accessible is the field as Huh's own input, for its plain prompts, which
// draw no cursor: titled in the field's theme, and checking what it is
// answered with as the field checks what is typed.
func (q *Input) accessible() *huh.Input {
	input := huh.NewInput().Title(q.title).Description(q.hint).Placeholder(q.placeholder).
		EchoMode(huh.EchoMode(q.input.EchoMode)).Value(q.value).Validate(q.validate)
	input.WithTheme(q.theme)

	return input
}

// textStyles are styles with the text typed drawn as theme draws a field's:
// its prompt, the text and the placeholder, focused and not, and the cursor
// in the theme's color.
func textStyles(styles textinput.Styles, theme *huh.Styles) textinput.Styles {
	styles.Focused = textStyleState(theme.Focused.TextInput)
	styles.Blurred = textStyleState(theme.Blurred.TextInput)
	styles.Cursor.Color = theme.Focused.TextInput.Cursor.GetForeground()

	return styles
}

// textStyleState is a theme's text styles as a text input holds them.
func textStyleState(text huh.TextInputStyles) textinput.StyleState {
	return textinput.StyleState{Placeholder: text.Placeholder, Prompt: text.Prompt, Text: text.Text}
}
