// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// quietInput is a Huh field typing one line of text, as huh.Input does, over
// an input whose cursor never blinks.
//
// Trade-off TRADE-46: huh.Input blinks its cursor and offers no way not to;
// each focus and each key schedules a blink that waits half a second of real
// time, which a test settling after every key pays in full.
type quietInput struct {
	title, description, placeholder string
	echo                            textinput.EchoMode
	value                           *string
	validate                        func(string) error
	input                           textinput.Model
	err                             error
	focused                         bool
	keymap                          huh.InputKeyMap
	theme                           huh.Theme
	width                           int
}

var _ huh.Field = (*quietInput)(nil)

// newQuietInput is a field titled title, with a hint under it and a
// placeholder in it until something is typed, holding what value holds and
// writing what is typed back to it.
func newQuietInput(title, description, placeholder string, value *string) *quietInput {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = placeholder
	input.SetValue(*value)

	return &quietInput{
		title: title, description: description, placeholder: placeholder, value: value, input: input,
		validate: func(string) error { return nil },
	}
}

// Init starts the field unfocused, as huh.Input does.
func (q *quietInput) Init() tea.Cmd {
	q.input.Blur()

	return nil
}

// Update answers a key or a paste: next-field, previous-field and submit move
// on once what was typed passes its check, and anything else is typed. The
// input's blink is never handed on.
//
//nolint:ireturn // huh.Field's Update returns Huh's Model
func (q *quietInput) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	press, isKey := msg.(tea.KeyPressMsg)
	if _, isPaste := msg.(tea.PasteMsg); !isKey && !isPaste {
		return q, nil
	}

	q.err = nil

	switch {
	case isKey && key.Matches(press, q.keymap.Prev):
		return q, huh.PrevField
	case isKey && key.Matches(press, q.keymap.Next, q.keymap.Submit):
		q.err = q.validate(q.input.Value())
		if q.err != nil {
			return q, nil
		}

		return q, huh.NextField
	}

	q.input, _ = q.input.Update(msg)
	*q.value = q.input.Value()

	return q, nil
}

// View is the field's title, its hint, and the line typed, in the theme's
// styles for a field focused or not.
func (q *quietInput) View() string {
	styles := q.styles()
	lines := []string{styles.Title.Render(q.title)}

	if q.description != "" {
		lines = append(lines, styles.Description.Render(q.description))
	}

	return styles.Base.Width(q.width).Render(strings.Join(append(lines, q.input.View()), "\n"))
}

// Blur leaves the field, keeping and checking what was typed.
func (q *quietInput) Blur() tea.Cmd {
	q.focused = false
	q.input.Blur()
	*q.value = q.input.Value()
	q.err = q.validate(*q.value)

	return nil
}

// Focus enters the field, dropping the blink the input asks for.
func (q *quietInput) Focus() tea.Cmd {
	q.focused = true
	q.input.Focus()

	return nil
}

// Error is why what was typed was refused, if it was.
func (q *quietInput) Error() error {
	return q.err
}

// Run asks the field on its own, as Huh's input.
func (q *quietInput) Run() error {
	return q.accessible().Run() //nolint:wrapcheck // Huh's own words for a field it could not ask
}

// RunAccessible asks the field as a plain prompt on in and out.
func (q *quietInput) RunAccessible(out io.Writer, in io.Reader) error {
	return q.accessible().RunAccessible(out, in) //nolint:wrapcheck // as Run
}

// Skip reports that the field is asked, never passed over.
func (q *quietInput) Skip() bool { return false }

// Zoom reports that the field shares its group's height.
func (q *quietInput) Zoom() bool { return false }

// KeyBinds are the keys the field answers, for the footer.
func (q *quietInput) KeyBinds() []key.Binding {
	return []key.Binding{q.keymap.Prev, q.keymap.Submit, q.keymap.Next}
}

// WithTheme draws the field in theme, unless it has one of its own.
//
//nolint:ireturn // huh.Field's own signature
func (q *quietInput) WithTheme(theme huh.Theme) huh.Field {
	if q.theme == nil {
		q.theme = theme
	}

	return q
}

// WithKeyMap keys the field by keymap's input keys.
//
//nolint:ireturn // huh.Field's own signature
func (q *quietInput) WithKeyMap(keymap *huh.KeyMap) huh.Field {
	q.keymap = keymap.Input

	return q
}

// WithWidth fits the field and its line to width columns.
//
//nolint:ireturn // huh.Field's own signature
func (q *quietInput) WithWidth(width int) huh.Field {
	q.width = width
	q.input.SetWidth(max(1, width-len(q.input.Prompt)-1))

	return q
}

// WithHeight leaves the field one line tall.
//
//nolint:ireturn // huh.Field's own signature
func (q *quietInput) WithHeight(int) huh.Field {
	return q
}

// WithPosition offers previous-field everywhere but the first field, and
// next-field everywhere but the last, which submit answers instead.
//
//nolint:ireturn // huh.Field's own signature
func (q *quietInput) WithPosition(position huh.FieldPosition) huh.Field {
	q.keymap.Prev.SetEnabled(!position.IsFirst())
	q.keymap.Next.SetEnabled(!position.IsLast())
	q.keymap.Submit.SetEnabled(position.IsLast())

	return q
}

// GetKey is the field's key in the form's results: it has none, since what
// it holds is written to its value.
func (q *quietInput) GetKey() string { return "" }

// GetValue is what was typed.
func (q *quietInput) GetValue() any { return q.input.Value() }

// checkedBy is the field refusing, with check's reason, what check refuses.
func (q *quietInput) checkedBy(check func(string) error) *quietInput {
	q.validate = check

	return q
}

// styles are the theme's styles for the field as it is now.
func (q *quietInput) styles() huh.FieldStyles {
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
// draw no cursor.
func (q *quietInput) accessible() *huh.Input {
	return huh.NewInput().Title(q.title).Description(q.description).Placeholder(q.placeholder).
		EchoMode(huh.EchoMode(q.echo)).Value(q.value).Validate(q.validate)
}
