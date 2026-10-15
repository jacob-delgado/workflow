// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// settingsEdit is a Huh form open over Settings' rows: a section's settings
// as one group, an entry's value, or a new entry's name and value. keep lays
// what it was answered with over the form, or says why it cannot be.
type settingsEdit struct {
	form *huh.Form
	keep func(settingsForm) (settingsForm, error)
}

// settingsKeep is what the footer calls keeping what a form open over
// Settings holds, which apply on its last field and save-settings on any do
// alike.
const settingsKeep = "keep"

// settingAnswer is a field's answer as it is filled in, beside what it
// started as, so a field left alone is no edit.
type settingAnswer struct {
	text, from string
	on, wasOn  bool
}

// settingsEditDone is a form over Settings answered, or, asked as prompts,
// the error that stopped them.
type settingsEditDone struct {
	form *huh.Form
	err  error
}

var _ applier = settingsEditDone{}

// apply keeps what the form was answered with. A form since closed or
// replaced keeps nothing.
func (msg settingsEditDone) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(settingsForm)
	if !open || form.edit == nil || form.edit.form != msg.form {
		return m, nil
	}

	m.overlay = form.keptEdit(msg.err)

	return m, nil
}

// keptEdit is Settings with the open form closed and what it was answered
// with kept, or with why it could not be.
func (f settingsForm) keptEdit(err error) settingsForm {
	if err == nil {
		f, err = f.edit.keep(f)
	}

	f.edit, f.problem = nil, err

	return f
}

// startEditing opens a form for the selected row: its section's settings, a
// collection entry's value, or, on the row that adds one, a new entry.
func (f settingsForm) startEditing(m Model) (Model, tea.Cmd) {
	field := f.current()
	groups, keep := f.groupsFor(field)

	// Its start asks for a size it is already given and for titles drawn
	// from functions none of its fields has: it has no use for either.
	f.edit = &settingsEdit{keep: keep}
	width, rows := f.editSize(m)
	form, _ := newHuhForm(m.kit(), huhKeys(m.keys, settingsKeep), width, rows, groups...)

	for range f.at(field) {
		form.NextField()
	}

	f.edit.form = form
	m.overlay = f

	return m, m.startForm(form, nil, func(err error) tea.Msg { return settingsEditDone{form: form, err: err} })
}

// groupsFor are the groups a row's form asks, and how what they are
// answered with is kept: a collection entry's value, a new entry, or, for
// any other row, its section's settings.
func (f settingsForm) groupsFor(field setting) ([]*huh.Group, func(settingsForm) (settingsForm, error)) {
	if field.kind == settingEntry {
		return f.entryGroups(field)
	}

	if field.kind == settingAdd {
		return f.addGroups(field)
	}

	return f.sectionGroups()
}

// editSize is the columns and rows a form open over Settings has: the
// pane's, less what Settings says above it and a blank row, and the outcome
// of the last key under it.
func (f settingsForm) editSize(m Model) (int, int) {
	width := m.detailWidth()
	around := len(f.head(width)) + 1 + len(splitLines(f.footLines(m.kit(), width)...))

	return width, max(1, m.detailRows()-around)
}

// fitEdit fits the form open over Settings to the rows it has now, which a
// refusal takes from: Settings' own, of a keep, under the form, and the
// form's, which Huh makes room for only when it is fitted while saying one.
func (f settingsForm) fitEdit(m Model) tea.Cmd {
	width, rows := f.editSize(m)

	return fitForm(f.edit.form, width, rows)
}

// at is how many fields into its section's form a setting is: none for an
// entry's own form.
func (f settingsForm) at(field setting) int {
	return slices.IndexFunc(f.sectionSettings(field.section), func(each setting) bool { return each.path == field.path })
}

// sectionSettings are the settings of a section a form edits whole: every
// one but a collection's rows.
func (f settingsForm) sectionSettings(section string) []setting {
	return slices.DeleteFunc(slices.Clone(f.fields), func(each setting) bool {
		return each.section != section || each.kind == settingEntry || each.kind == settingAdd
	})
}

// sectionGroups are the selected row's section as one group, a field to a
// setting, and how what it is answered with is kept.
func (f settingsForm) sectionGroups() ([]*huh.Group, func(settingsForm) (settingsForm, error)) {
	settings := f.sectionSettings(f.current().section)
	answers := make([]*settingAnswer, len(settings))
	fields := make([]huh.Field, len(settings))

	for index, each := range settings {
		answers[index] = f.answerFor(each)
		fields[index] = settingField(each, answers[index])
		// A choice's field writes the option it opens on into its answer,
		// which is not what was read where the file spells it another way,
		// so the answer starts from what the field holds.
		answers[index].from = answers[index].text
	}

	keep := func(into settingsForm) (settingsForm, error) { return into.keptSection(settings, answers) }

	return []*huh.Group{huh.NewGroup(fields...)}, keep
}

// answerFor is a setting's answer as its field starts: on or off as it is,
// a credential empty, and any other as it is shown.
func (f settingsForm) answerFor(field setting) *settingAnswer {
	isOn, _ := f.value(field.path).(bool)
	text := f.editedText(field)

	if field.kind == settingChoice {
		text = f.text(field)
	}

	return &settingAnswer{text: text, on: isOn, wasOn: isOn}
}

// settingField is the Huh field a setting is answered in: on or off for a
// toggle, one of its choices for a choice, and typed for any other kind a
// section's form holds, a credential masked and a count refused until it is
// one.
//
//nolint:ireturn // a Huh group holds any of its field kinds behind huh.Field
func settingField(field setting, answer *settingAnswer) huh.Field {
	if field.kind == settingToggle {
		return huh.NewConfirm().Title(field.label).Description(field.hint).Affirmative("On").Negative("Off").
			Inline(true).Value(&answer.on)
	}

	if field.kind == settingChoice {
		options := make([]huh.Option[string], len(field.choices))
		for index, choice := range field.choices {
			options[index] = huh.NewOption(choice.words, choice.value)
		}

		return huh.NewSelect[string]().Title(field.label).Description(field.hint).Options(options...).
			Value(&answer.text)
	}

	input := quietinput.New(field.label, "", &answer.text).Hinted(field.hint).CheckedBy(func(text string) error {
		_, _, err := typedValue(field, text)

		return err
	})
	if field.kind == settingSecret {
		return input.Secret()
	}

	return input
}

// keptSection is the form with every setting of a section answered other
// than it started kept as an edit, or with why one cannot be.
func (f settingsForm) keptSection(settings []setting, answers []*settingAnswer) (settingsForm, error) {
	for index, field := range settings {
		answer := answers[index]

		if field.kind == settingToggle {
			if answer.on != answer.wasOn {
				f = f.with(field.path, answer.on)
			}

			continue
		}

		value, changes, err := typedValue(field, answer.text)
		if err != nil {
			return f, err
		}

		if changes && answer.text != answer.from {
			f = f.with(field.path, value)
		}
	}

	// Laid out again, since a hint can name a setting turned on or off.
	return f.laidOut(), nil
}

// entryGroups ask a collection entry's value: a credential's empty and
// masked, and a key's or any other's as it is.
func (f settingsForm) entryGroups(field setting) ([]*huh.Group, func(settingsForm) (settingsForm, error)) {
	answer := &settingAnswer{}
	if !field.list.secret {
		answer.text = f.entryValue(field)
	}

	input := quietinput.New(field.label, field.list.valueWord, &answer.text).Hinted(field.hint)
	if field.list.secret {
		input = input.Secret()
	}

	keep := func(into settingsForm) (settingsForm, error) { return into.keptEntry(field, answer.text), nil }

	return []*huh.Group{huh.NewGroup(input)}, keep
}

// addGroups ask a new entry's name and its value together, refusing a name
// left empty or already listed and a value left empty.
func (f settingsForm) addGroups(field setting) ([]*huh.Group, func(settingsForm) (settingsForm, error)) {
	name, value := &settingAnswer{}, &settingAnswer{}
	named := quietinput.New(capitalized(field.list.nameWord), "", &name.text).
		CheckedBy(func(typed string) error { return f.nameProblem(field, typed) })
	valued := quietinput.New(capitalized(field.list.valueWord), "", &value.text).CheckedBy(valueProblem)

	if field.list.secret {
		valued = valued.Secret()
	}

	keep := func(into settingsForm) (settingsForm, error) { return into.added(field, name.text, value.text) }

	return []*huh.Group{huh.NewGroup(named, valued).Title(field.label).Description(field.hint)}, keep
}

// capitalized is a word as a field's title begins it.
func capitalized(word string) string {
	return strings.ToUpper(word[:1]) + word[1:]
}

// editKey answers a key while a form is open over Settings: esc closes it,
// keeping nothing; save-settings keeps what it holds from whichever field has
// the keyboard, as apply on its last field does, and a second save writes it;
// and every other key is the form's.
func (f settingsForm) editKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		f.edit, f.problem = nil, nil
	case key.Matches(msg, m.keys.saveSettings):
		kept, err := f.edit.keep(f)
		if err != nil {
			f.problem = err
			m.overlay = f

			return m, f.fitEdit(m)
		}

		f, f.edit = kept, nil
	default:
		f.problem = nil
		m.overlay = f

		return f.relay(m, msg)
	}

	m.overlay = f

	return m, nil
}
