// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errNeedsValue reports a one-line write left empty.
var errNeedsValue = errors.New("needs a value")

// errNeedsJira says which field keeps a transition out of reach here.
func errNeedsJira(move jira.Transition, field jira.Field) error {
	return fmt.Errorf("%s needs %s, %w", move.Name, field.Name, jira.ErrOnlyJira)
}

// fieldForm fills in the fields a transition needs, one at a time, in a Huh
// form: a choice from a field's allowed values, several of them for a list,
// or text typed for a text, user or date field. Each field is a group of its
// own, so the form shows one at a time and says how far along it is.
type fieldForm struct {
	transition jira.Transition
	form       *huh.Form
	answers    []*fieldAnswer
	// last is the last field, and submitting is set once apply on it has
	// passed its check: the form takes nothing more while its move goes.
	last       huh.Field
	submitting bool
}

// fieldAnswer is what a field holds as it is filled in, bound to its Huh
// field: typed text, a chosen option, or the options chosen for a list.
type fieldAnswer struct {
	text    string
	option  string
	options []string
}

// fieldsFilled is the form's last field filled in, the transition can go; or,
// asked as prompts, the error that stopped them.
type fieldsFilled struct {
	form *huh.Form
	err  error
}

var _ applier = fieldsFilled{}

// newFieldForm starts filling in a transition's fields, drawn with kit, keyed
// by keys and sized to width columns and rows rows.
func newFieldForm(move jira.Transition, kit renderKit, keys keyMap, width, rows int) (fieldForm, tea.Cmd) {
	answers := make([]*fieldAnswer, len(move.Fields))
	groups := make([]*huh.Group, len(move.Fields))

	var last huh.Field

	for index, field := range move.Fields {
		answers[index] = &fieldAnswer{}
		title := field.Name + " (" + strconv.Itoa(index+1) + " of " + strconv.Itoa(len(move.Fields)) + ")"
		last = huhField(field, title, answers[index])
		groups[index] = huh.NewGroup(last)
	}

	form, start := newHuhForm(kit, keys, width, rows, groups...)

	return fieldForm{transition: move, form: form, answers: answers, last: last}, start
}

// huhField is the Huh field that fills in one of a transition's fields into
// answer, refusing what Jira's own field checks refuse: text typed for a text,
// user or date field, several options for a list, and one option for any
// other kind a form can fill. Each check is of the answer Huh hands it, which
// a plain prompt checks before it keeps it in answer.
//
//nolint:ireturn // a Huh group holds any of its field kinds behind huh.Field
func huhField(field jira.Field, title string, answer *fieldAnswer) huh.Field {
	switch {
	case textual(field.Kind):
		return newQuietInput(title, placeholder(field.Kind), &answer.text).
			checkedBy(func(text string) error { return (&fieldAnswer{text: text}).checked(field) })
	case field.Kind == jira.FieldOptionList:
		// Its height is named: left to itself, Huh counts its options' rows and
		// takes the title out of them, drawing one option too few.
		return huh.NewMultiSelect[string]().Title(title).Options(huhOptions(field)...).Value(&answer.options).
			Height(len(field.Options) + 1).
			Validate(func(options []string) error { return (&fieldAnswer{options: options}).checked(field) })
	default:
		return huh.NewSelect[string]().Title(title).Options(huhOptions(field)...).Value(&answer.option).
			Validate(func(option string) error { return (&fieldAnswer{option: option}).checked(field) })
	}
}

// huhOptions are a field's allowed values, as Huh lists them.
func huhOptions(field jira.Field) []huh.Option[string] {
	options := make([]huh.Option[string], len(field.Options))
	for index, option := range field.Options {
		options[index] = huh.NewOption(option.Name, option.ID)
	}

	return options
}

// textual reports whether a field of the kind is filled by typing rather than
// by choosing from a list.
func textual(kind jira.FieldKind) bool {
	return map[jira.FieldKind]bool{
		jira.FieldText:        true,
		jira.FieldUser:        true,
		jira.FieldDate:        true,
		jira.FieldUnsupported: false,
		jira.FieldOption:      false,
		jira.FieldOptionList:  false,
	}[kind]
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

// open reports whether a form is being filled in.
func (f fieldForm) open() bool {
	return f.form != nil
}

// view draws the field being filled in under what it is for.
func (f fieldForm) view(kit renderKit) []string {
	return []string{transitionLabel(kit.marks, f.transition) + " needs:", f.form.View()}
}

// footer offers what the field being filled in answers, and says that esc
// steps back to the transitions.
func (f fieldForm) footer(keys keyMap) []key.Binding {
	return append(formKeys(f.form), relabel(keys.closeOverlay, escBack))
}

// handleFormKey answers a key while a transition's fields are being filled in:
// esc goes back to the transitions rather than closing the picker, and every
// other key is the form's, until apply on the last field passes its check.
func (p statusPicker) handleFormKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case p.form.submitting:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		p.form = fieldForm{}
		m.overlay = p

		return m, nil
	}

	onLast := p.form.form.GetFocusedField() == p.form.last
	m, cmd := p.relay(m, msg)
	p.form.submitting = onLast && key.Matches(msg, m.keys.confirm) && p.form.last.Error() == nil
	m.overlay = p

	return m, cmd
}

// relay hands a message to the form while a transition's fields are being
// filled in.
func (p statusPicker) relay(m Model, msg tea.Msg) (Model, tea.Cmd) {
	if !p.form.open() {
		return m, nil
	}

	return m, relayToForm(p.form.form, msg)
}

// resized fits the form being filled in, if one is, to the picker at the
// screen's new size.
func (p statusPicker) resized(m Model) (Model, tea.Cmd) {
	if !p.form.open() {
		return m, nil
	}

	width, rows := p.formSize(m)

	return m, fitForm(p.form.form, width, rows)
}

// formSize is the columns and rows a transition's form has in the picker:
// the pane's, less the picker's header and the line naming the move.
func (p statusPicker) formSize(m Model) (int, int) {
	return m.detailWidth(), m.detailRows() - len(p.header()) - 1
}

// pasted types a paste into a transition's text field while one is filled in,
// and not once its last field has been applied.
func (p statusPicker) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if p.form.submitting {
		return m, nil
	}

	return p.relay(m, paste)
}

// step moves the choice by delta: through a field's options, as its bound up
// or down would, while a form is open, and through the transitions
// otherwise. Nothing moves while a change is being sent.
func (p statusPicker) step(m Model, delta int) Model {
	switch {
	case p.send.sending:
		return m
	case p.form.open():
		toward := m.keys.down
		if delta < 0 {
			toward = m.keys.up
		}

		for range max(delta, -delta) {
			m, _ = p.relay(m, pressOf(toward))
		}

		return m
	}

	p.transitions = p.transitions.moved(delta)
	m.overlay = p

	return m
}

// apply sends the transition the form was filled in for, with the values it
// holds, once its last field is filled, closing the form first so the picker
// shows the move under way. A form since closed or replaced sends nothing.
func (msg fieldsFilled) apply(m Model) (Model, tea.Cmd) {
	picker, open := m.overlay.(statusPicker)
	if !open || picker.form.form != msg.form {
		return m, nil
	}

	problem := msg.err
	if problem == nil {
		problem = picker.form.problem()
	}

	if problem != nil {
		m.overlay = picker.failed(problem)

		return m, nil
	}

	move, values := picker.form.transition, picker.form.values()
	picker.form = fieldForm{}

	return picker.apply(m, move, values)
}

// problem is the first field whose answer cannot be accepted, nil once
// every one can: prompts that ended early leave some unanswered.
func (f fieldForm) problem() error {
	for index, field := range f.transition.Fields {
		err := f.answers[index].checked(field)
		if err != nil {
			return err
		}
	}

	return nil
}

// values are what was entered for every field, in the shape each kind needs.
func (f fieldForm) values() []jira.FieldValue {
	values := make([]jira.FieldValue, len(f.transition.Fields))
	for index, field := range f.transition.Fields {
		values[index] = f.answers[index].value(field)
	}

	return values
}

// value is what was entered for field, in the shape its kind needs.
func (a *fieldAnswer) value(field jira.Field) jira.FieldValue {
	value := jira.FieldValue{Field: field}

	switch {
	case textual(field.Kind):
		value.Text = strings.TrimSpace(a.text)
	case field.Kind == jira.FieldOptionList:
		value.OptionIDs = a.options
	default:
		value.OptionID = a.option
	}

	return value
}

// checked is the reason what was entered for field cannot be accepted yet, as
// Jira's own field checks give it, named for the field; nil once it can.
func (a *fieldAnswer) checked(field jira.Field) error {
	err := a.value(field).Check()
	if err != nil {
		return fmt.Errorf("%s %w", field.Name, err)
	}

	return nil
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
