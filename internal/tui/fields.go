// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errOnlyJira reports a field only Jira's own screen can fill: a user picker, a
// date. The transition is not sent half-filled, to be refused.
var errOnlyJira = errors.New("which only Jira's own screen can fill — make this change in Jira")

// errNeedsValue reports a text field left empty.
var errNeedsValue = errors.New("needs a value")

// errNeedsJira says which field keeps a transition out of reach here.
func errNeedsJira(move jira.Transition, field jira.Field) error {
	return fmt.Errorf("%s needs %s, %w", move.Name, field.Name, errOnlyJira)
}

// unfillableField is the first field of a transition that cannot be filled in
// here, if it has one.
func unfillableField(move jira.Transition) (jira.Field, bool) {
	for _, field := range move.Fields {
		if !field.Fillable() {
			return field, true
		}
	}

	return jira.Field{}, false
}

// fieldForm fills in the fields a transition needs, one at a time: a choice from
// a field's allowed values, or text typed for a text field.
type fieldForm struct {
	transition jira.Transition
	index      int
	values     []jira.FieldValue
	option     int
	input      textinput.Model
	problem    error
}

// newFieldForm starts filling in a transition's fields.
func newFieldForm(move jira.Transition) fieldForm {
	return fieldForm{transition: move, index: 0, values: nil, option: 0, input: newInput(""), problem: nil}
}

// newInput is a focused one-line text input holding text. Its cursor does not
// blink: a blinking cursor is a timer running for the life of the input.
func newInput(text string) textinput.Model {
	input := textinput.New()
	input.Prompt = "> "
	input.SetValue(text)
	input.Cursor.SetMode(cursor.CursorStatic)
	input.Focus()

	return input
}

// open reports whether a form is being filled in.
func (f fieldForm) open() bool {
	return len(f.transition.Fields) > 0
}

// field is the field being filled in.
func (f fieldForm) field() jira.Field {
	return f.transition.Fields[f.index]
}

// view draws the field being filled in.
func (f fieldForm) view(marks glyphs, width, rows int) []string {
	field := f.field()
	lines := []string{
		transitionLabel(marks, f.transition) + " needs:",
		field.Name + " (" + strconv.Itoa(f.index+1) + " of " + strconv.Itoa(len(f.transition.Fields)) + ")",
	}

	if field.Kind == jira.FieldText {
		f.input.Width = max(1, width-len(f.input.Prompt)-1)
		lines = append(lines, f.input.View())
	} else {
		first, last := window(f.option, len(field.Options), rows-len(lines)-1)
		for index := first; index < last; index++ {
			lines = append(lines, marks.marker(index == f.option)+field.Options[index].Name)
		}
	}

	if f.problem != nil {
		lines = append(lines, marks.failed+" "+field.Name+" "+f.problem.Error())
	}

	return lines
}

// footer offers what works on the field being filled in, and says what esc and
// enter do here: esc steps back to the transitions, and enter moves to the next
// field until the last one, which it applies.
func (f fieldForm) footer(keys keyMap) []key.Binding {
	advance := relabel(keys.confirm, f.confirmLabel())
	back := relabel(keys.closeOverlay, "back")

	if f.field().Kind == jira.FieldText {
		return []key.Binding{advance, back}
	}

	return []key.Binding{keys.up, keys.down, advance, back}
}

// confirmLabel names enter for what it does on this field: apply on the last,
// otherwise on to the next.
func (f fieldForm) confirmLabel() string {
	if f.index == len(f.transition.Fields)-1 {
		return "apply"
	}

	return "next"
}

// handleFormKey answers a key while a transition's fields are being filled in.
// esc goes back to the transitions rather than closing the picker.
func (p statusPicker) handleFormKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	field := p.form.field()

	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		p.form = fieldForm{}
	case key.Matches(msg, m.keys.confirm):
		return p.fill(m)
	case field.Kind == jira.FieldText:
		p.form.input, _ = p.form.input.Update(msg)
		p.form.problem = nil
	case key.Matches(msg, m.keys.down):
		p.form.option = min(p.form.option+1, len(field.Options)-1)
	case key.Matches(msg, m.keys.up):
		p.form.option = max(0, p.form.option-1)
	}

	m.overlay = p

	return m, nil
}

// fill records the field's value and moves to the next field, or applies the
// transition once every field has one.
func (p statusPicker) fill(m Model) (Model, tea.Cmd) {
	field := p.form.field()
	value := jira.FieldValue{Field: field, OptionID: "", Text: ""}

	if field.Kind == jira.FieldText {
		value.Text = strings.TrimSpace(p.form.input.Value())
		if value.Text == "" {
			p.form.problem = errNeedsValue
			m.overlay = p

			return m, nil
		}
	} else {
		value.OptionID = field.Options[p.form.option].ID
	}

	p.form.values = append(p.form.values, value)
	p.form.index++
	p.form.option, p.form.input, p.form.problem = 0, newInput(""), nil

	if p.form.index < len(p.form.transition.Fields) {
		m.overlay = p

		return m, nil
	}

	// The form closes before sending, so the picker shows the move under way
	// rather than a field past the last one.
	move, values := p.form.transition, p.form.values
	p.form = fieldForm{}

	return p.apply(m, move, values)
}
