// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errNeedsValue reports a one-line write left empty.
var errNeedsValue = errors.New("needs a value")

// errNeedsJira says which field keeps a transition out of reach here.
func errNeedsJira(move jira.Transition, field jira.Field) error {
	return fmt.Errorf("%s needs %s, %w", move.Name, field.Name, jira.ErrOnlyJira)
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
	return map[jira.FieldKind]bool{
		jira.FieldText:        true,
		jira.FieldUser:        true,
		jira.FieldDate:        true,
		jira.FieldUnsupported: false,
		jira.FieldOption:      false,
		jira.FieldOptionList:  false,
	}[f.field().Kind]
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
		return jira.DateShape
	}

	if kind == jira.FieldUser {
		return "username"
	}

	return ""
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
func (f fieldForm) view(kit renderKit, width, rows int) []string {
	field := f.field()
	lines := []string{
		transitionLabel(kit.marks, f.transition) + " needs:",
		field.Name + " (" + strconv.Itoa(f.index+1) + " of " + strconv.Itoa(len(f.transition.Fields)) + ")",
	}

	if f.textual() {
		input := f.input
		input.Placeholder = placeholder(field.Kind)
		input.SetWidth(max(1, width-len(input.Prompt)-1))
		lines = append(lines, input.View())
	} else {
		lines = append(lines, f.optionLines(kit.marks, rows-len(lines)-1)...)
	}

	if f.problem != nil {
		lines = append(lines, kit.failureLine(fmt.Errorf("%s %w", field.Name, f.problem)))
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
			row += checkbox(f.picked(options[index].ID))
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
	back := relabel(keys.closeOverlay, escBack)

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
	case key.Matches(msg, m.keys.cursorKeys()...):
		return p.step(m, m.keys.stepOf(msg)), nil
	}

	m.overlay = p

	return m, nil
}

// pasted types a paste into a transition's text field while one is filled in.
// The form closes before its move is sent, so none is open while one is out.
func (p statusPicker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if !p.form.open() || !p.form.textual() {
		return m, nil
	}

	p.form.input, _ = p.form.input.Update(paste)
	p.form.problem = nil
	m.overlay = p

	return m, nil
}

// step moves the choice by delta: through the field's options while a
// transition's fields are filled in, and through the transitions otherwise.
// Nothing moves while a change is being sent.
func (p statusPicker) step(m Model, delta int) Model {
	switch {
	case p.send.sending:
		return m
	case p.form.open():
		// A field typed in reads its input and draws no options, so the choice
		// moving there changes nothing.
		p.form.option = max(0, min(p.form.option+delta, len(p.form.field().Options)-1))
	default:
		p.transitions = p.transitions.moved(delta)
	}

	m.overlay = p

	return m
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
// be accepted yet, as Jira's own field checks give it.
func (f fieldForm) value() (jira.FieldValue, error) {
	field := f.field()
	value := jira.FieldValue{Field: field}

	switch {
	case f.textual():
		value.Text = strings.TrimSpace(f.input.Value())
	case f.multi():
		value.OptionIDs = f.chosen
	default:
		value.OptionID = field.Options[f.option].ID
	}

	return value, value.Check()
}
