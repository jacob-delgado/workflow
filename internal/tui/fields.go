// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errOnlyJira reports a field only Jira's own screen can fill, such as a
// cascading select. The transition is not sent half-filled, to be refused.
var errOnlyJira = errors.New("which only Jira's own screen can fill; make this change in Jira")

// errNeedsValue reports a text, user or date field left empty.
var errNeedsValue = errors.New("needs a value")

// errNeedsDate reports a date field that is not a date.
var errNeedsDate = errors.New("must be a date like 2026-09-21")

// errNeedsChoice reports a list field with nothing chosen.
var errNeedsChoice = errors.New("needs at least one")

// dateLayout is the calendar date Jira reads and writes, and Go's own reference
// date spelled in it.
const dateLayout = "2006-01-02"

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

// fieldForm fills in the fields a transition needs, one at a time: a choice
// from a field's allowed values, several of them for a list, or text typed for
// a text, user or date field.
type fieldForm struct {
	transition jira.Transition
	index      int
	values     []jira.FieldValue
	option     int
	chosen     []string
	input      textinput.Model
	problem    error
}

// newFieldForm starts filling in a transition's fields.
func newFieldForm(move jira.Transition) fieldForm {
	return fieldForm{
		transition: move, index: 0, values: nil, option: 0, chosen: nil, input: newInput(""), problem: nil,
	}
}

// textual reports whether the field is filled by typing rather than by
// choosing from a list.
func (f fieldForm) textual() bool {
	switch f.field().Kind {
	case jira.FieldText, jira.FieldUser, jira.FieldDate:
		return true
	case jira.FieldUnsupported, jira.FieldOption, jira.FieldOptionList:
		return false
	default:
		return false
	}
}

// multi reports whether the field takes any number of its options.
func (f fieldForm) multi() bool {
	return f.field().Kind == jira.FieldOptionList
}

// picked reports whether an option is among those chosen for a list field.
func (f fieldForm) picked(id string) bool {
	return slices.Contains(f.chosen, id)
}

// placeholder is the shape a typed field expects, shown until it is filled.
func placeholder(kind jira.FieldKind) string {
	if kind == jira.FieldDate {
		return dateLayout
	}

	if kind == jira.FieldUser {
		return "username"
	}

	return ""
}

// isDate reports whether text is a calendar date Jira will accept.
func isDate(text string) bool {
	_, err := time.Parse(dateLayout, text)

	return err == nil
}

// toggleID adds an option to the chosen set if absent and removes it if
// present, returning a new slice so a copied form never shares the old one's
// backing array.
func toggleID(ids []string, id string) []string {
	if slices.Contains(ids, id) {
		return slices.DeleteFunc(slices.Clone(ids), func(each string) bool { return each == id })
	}

	return append(slices.Clone(ids), id)
}

// newInput is a focused one-line text input holding text. Its cursor does not
// blink: a blinking cursor is a timer running for the life of the input.
func newInput(text string) textinput.Model {
	input := textinput.New()
	input.Prompt = "> "
	input.SetValue(text)
	// The virtual cursor draws in reverse video inline, which is what the screen
	// wants; it does not blink because the blink command Focus returns is dropped
	// here rather than run, so no timer runs for the life of the input.
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
func (f fieldForm) view(marks glyphs, sty styles, width, rows int) []string {
	field := f.field()
	lines := []string{
		transitionLabel(marks, f.transition) + " needs:",
		field.Name + " (" + strconv.Itoa(f.index+1) + " of " + strconv.Itoa(len(f.transition.Fields)) + ")",
	}

	if f.textual() {
		input := f.input
		input.Placeholder = placeholder(field.Kind)
		input.SetWidth(max(1, width-len(input.Prompt)-1))
		lines = append(lines, input.View())
	} else {
		lines = append(lines, f.optionLines(marks, rows-len(lines)-1)...)
	}

	if f.problem != nil {
		lines = append(lines, failedGlyph(sty, marks)+" "+field.Name+" "+f.problem.Error())
	}

	return lines
}

// optionLines draws a field's values, a checkbox before each for a list field
// so the ones chosen so far are plain to see.
func (f fieldForm) optionLines(marks glyphs, rows int) []string {
	options := f.field().Options
	first, last := window(f.option, len(options), rows)

	lines := make([]string, 0, last-first)
	for index := first; index < last; index++ {
		row := marks.marker(index == f.option)
		if f.multi() {
			row += marks.checkbox(f.picked(options[index].ID))
		}

		lines = append(lines, row+options[index].Name)
	}

	return lines
}

// footer offers what works on the field being filled in, and says what esc and
// enter do here: esc steps back to the transitions, and enter moves to the next
// field until the last one, which it applies.
func (f fieldForm) footer(keys keyMap) []key.Binding {
	advance := relabel(keys.confirm, f.confirmLabel())
	back := relabel(keys.closeOverlay, "back")

	if f.textual() {
		return []key.Binding{advance, back}
	}

	if f.multi() {
		return []key.Binding{keys.up, keys.down, keys.toggleOption, advance, back}
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
func (p statusPicker) handleFormKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	field := p.form.field()

	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		p.form = fieldForm{}
	case key.Matches(msg, m.keys.confirm):
		return p.fill(m)
	case p.form.textual():
		p.form.input, _ = p.form.input.Update(msg)
		p.form.problem = nil
	case p.form.multi() && key.Matches(msg, m.keys.toggleOption):
		p.form.chosen = toggleID(p.form.chosen, field.Options[p.form.option].ID)
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
	value, problem := p.form.value()
	if problem != nil {
		p.form.problem = problem
		m.overlay = p

		return m, nil
	}

	p.form.values = append(p.form.values, value)
	p.form.index++
	p.form.option, p.form.chosen, p.form.input, p.form.problem = 0, nil, newInput(""), nil

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

// value reads what was entered for the current field, or the reason it cannot
// be accepted yet.
func (f fieldForm) value() (jira.FieldValue, error) {
	field := f.field()
	value := jira.FieldValue{Field: field}

	switch {
	case f.textual():
		return f.typed(value)
	case f.multi():
		if len(f.chosen) == 0 {
			return value, errNeedsChoice
		}

		value.OptionIDs = f.chosen
	default:
		value.OptionID = field.Options[f.option].ID
	}

	return value, nil
}

// typed reads a text, user or date field, refusing an empty value and a date
// that is not one.
func (f fieldForm) typed(value jira.FieldValue) (jira.FieldValue, error) {
	text := strings.TrimSpace(f.input.Value())
	if text == "" {
		return value, errNeedsValue
	}

	if f.field().Kind == jira.FieldDate && !isDate(text) {
		return value, errNeedsDate
	}

	value.Text = text

	return value, nil
}
