// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// The refusals adding an entry says in its own words.
var (
	errEntryUnnamed = errors.New("an entry needs a name")
	errEntryTaken   = errors.New("an entry of that name is already listed")
	errEntryEmpty   = errors.New("an entry needs a value")
)

// entryName is the key a list's entry is named under, and the word a name is
// asked for with.
const entryName = "name"

// rebindNotAllowed is the one action ui.keys cannot move: its keys are the pane
// numbers, which no one key can stand in for, and CheckKeys refuses it.
const rebindNotAllowed = actionJumpToPane

// collection is a setting that is a list of named entries: the Jira views, a
// list of objects; or a map — the headers, the branch prefixes, the keys. Each
// entry is a row of Settings, after it a row that adds one, except for the
// keys, whose names are the actions and are all listed.
type collection struct {
	// title begins each entry's row; noun names one entry in the row that adds
	// one; the name and value words ask for its two parts.
	title, noun, nameWord, valueWord string
	// field is the key each entry's value is under, for a list of objects; a
	// map has none.
	field string
	// secret marks entries whose values are credentials: never shown, and kept
	// when left empty.
	secret bool
	// actions, for the keys, are the actions listed, with the key each keeps.
	actions []seams.KeyAction
	// matchedBy is the form two names are one in, as the configuration matches
	// them: a header's canonical form, an issue type lowered; nil holds names
	// apart unless they are equal.
	matchedBy func(string) string
}

// entry is one entry of a collection: its name, and its value as text.
type entry struct {
	name, value string
}

// entries are the collection's entries in a value of its JSON form: a list's
// in its order, a map's by name, or every action's for the keys.
func (c collection) entries(value any) []entry {
	if c.field != "" {
		listed, _ := value.([]any)
		read := make([]entry, 0, len(listed))

		for _, item := range listed {
			object, _ := item.(map[string]any)
			name, _ := object[entryName].(string)
			text, _ := object[c.field].(string)
			read = append(read, entry{name: name, value: text})
		}

		return read
	}

	mapped, _ := value.(map[string]any)
	read := make([]entry, 0, len(mapped))

	for _, name := range slices.Sorted(maps.Keys(mapped)) {
		text, _ := mapped[name].(string)
		read = append(read, entry{name: name, value: text})
	}

	return read
}

// value is the entries in the collection's JSON form, or nil for none, as a
// file with none writes it. A key left empty keeps its default, so it is left
// out of the map.
func (c collection) value(entries []entry) any {
	if len(entries) == 0 {
		return nil
	}

	if c.field != "" {
		listed := make([]any, 0, len(entries))
		for _, each := range entries {
			listed = append(listed, map[string]any{entryName: each.name, c.field: each.value})
		}

		return listed
	}

	mapped := make(map[string]any, len(entries))
	for _, each := range entries {
		mapped[each.name] = each.value
	}

	return mapped
}

// rows are a collection's rows under section: one per entry, then one that
// adds an entry; for the keys, one per action.
func (c collection) rows(section, path, hint string, value any) []setting {
	if c.actions != nil {
		return c.keyRows(section, path, hint)
	}

	rows := []setting{}
	for _, each := range c.entries(value) {
		rows = append(rows, setting{
			section: section, label: c.title + " " + sanitize.Line(each.name), path: path, hint: hint,
			kind: settingEntry, entry: each.name, list: c,
		})
	}

	return append(rows, setting{
		section: section, label: "Add a " + c.noun, path: path, hint: hint, kind: settingAdd, list: c,
	})
}

// keyRows are a row per action ui.keys can move.
func (c collection) keyRows(section, path, hint string) []setting {
	rows := []setting{}

	for _, action := range c.actions {
		if action.Action == rebindNotAllowed {
			continue
		}

		rows = append(rows, setting{
			section: section, label: action.Action, path: path, hint: hint, kind: settingEntry,
			entry: action.Action, list: c,
		})
	}

	return rows
}

// defaultKey is the key an action keeps with no ui.keys entry.
func (c collection) defaultKey(action string) string {
	index := slices.IndexFunc(c.actions, func(listed seams.KeyAction) bool { return listed.Action == action })
	if index < 0 {
		return ""
	}

	return c.actions[index].Default
}

// entryValue is an entry's value as the form holds it: empty when it is not
// there, as a key with no entry is not.
func (f settingsForm) entryValue(field setting) string {
	for _, each := range field.list.entries(f.value(field.path)) {
		if each.name == field.entry {
			return each.value
		}
	}

	return ""
}

// shownEntry is an entry's value as the screen may show it: a credential
// masked as read, or said to be new; a key, or the default it keeps.
func (f settingsForm) shownEntry(field setting) string {
	value := f.entryValue(field)

	switch {
	case field.list.actions != nil && value == "":
		return field.list.defaultKey(field.entry) + " (default)"
	case field.list.secret:
		read := field.list.entries(valueAt(f.values, field.path))
		if slices.Contains(read, entry{name: field.entry, value: value}) {
			return value
		}

		return "new value, hidden"
	}

	return sanitize.Line(value)
}

// withEntries is the form with a collection's entries replaced, its rows laid
// out again.
func (f settingsForm) withEntries(field setting, entries []entry) settingsForm {
	return f.with(field.path, field.list.value(entries)).laidOut()
}

// keptEntry is the form with the value typed for an entry kept: a credential
// left empty keeps the one stored, and a key left empty its default.
func (f settingsForm) keptEntry(field setting, typed string) settingsForm {
	if field.list.secret && typed == "" {
		return f
	}

	entries := field.list.entries(f.value(field.path))
	index := slices.IndexFunc(entries, func(each entry) bool { return each.name == field.entry })

	switch {
	case typed == "" && index >= 0:
		entries = slices.Delete(entries, index, index+1)
	case index >= 0:
		entries[index].value = typed
	case typed != "":
		entries = append(entries, entry{name: field.entry, value: typed})
	}

	return f.withEntries(field, entries)
}

// nameProblem is why a new entry cannot be named typed: left empty, or
// named as one listed; nil once it can.
func (f settingsForm) nameProblem(field setting, typed string) error {
	name := strings.TrimSpace(typed)
	listed := field.list.entries(f.value(field.path))

	switch {
	case name == "":
		return errEntryUnnamed
	case slices.ContainsFunc(listed, func(each entry) bool { return field.list.sameName(each.name, name) }):
		return errEntryTaken
	default:
		return nil
	}
}

// valueProblem is why a new entry cannot have the value typed: it is left
// empty.
func valueProblem(typed string) error {
	if strings.TrimSpace(typed) == "" {
		return errEntryEmpty
	}

	return nil
}

// sameName reports that two names are one entry's.
func (c collection) sameName(listed, typed string) bool {
	if c.matchedBy == nil {
		return listed == typed
	}

	return c.matchedBy(listed) == c.matchedBy(typed)
}

// added is the form with a new entry of the name and value typed, or with
// why it cannot be. A credential's value is kept as typed, spaces and all.
func (f settingsForm) added(field setting, name, typed string) (settingsForm, error) {
	err := f.nameProblem(field, name)
	if err == nil {
		err = valueProblem(typed)
	}

	if err != nil {
		return f, err
	}

	value := strings.TrimSpace(typed)
	if field.list.secret {
		value = typed
	}

	entries := append(field.list.entries(f.value(field.path)), entry{name: strings.TrimSpace(name), value: value})

	return f.withEntries(field, entries), nil
}

// removedEntry is the form without the entry the cursor is on: an edit like
// any other, which a save writes. A key's entry goes back to its default.
func (f settingsForm) removedEntry(field setting) settingsForm {
	entries := slices.DeleteFunc(field.list.entries(f.value(field.path)), func(each entry) bool {
		return each.name == field.entry
	})

	return f.withEntries(field, entries)
}

// isEdited reports that a setting was changed since the read: an entry when
// its value is not the one read, or it was not read at all.
func (f settingsForm) isEdited(field setting) bool {
	if field.kind == settingAdd {
		return false
	}

	if field.kind != settingEntry {
		_, edited := f.edits[field.path]

		return edited
	}

	return f.entryState(field, f.value(field.path)) != f.entryState(field, valueAt(f.values, field.path))
}

// entryState is an entry as a value of its collection holds it: its value,
// and whether it is there at all.
func (f settingsForm) entryState(field setting, value any) entry {
	for _, each := range field.list.entries(value) {
		if each.name == field.entry {
			return each
		}
	}

	return entry{}
}

// headerCollection is the Jira headers: names and credentials.
func headerCollection() collection {
	return collection{
		title: "Header", noun: "header", nameWord: entryName, valueWord: "value", secret: true,
		matchedBy: http.CanonicalHeaderKey,
	}
}

// keysOf is a configuration's keys collection, from the actions the
// interface binds.
func keysOf(actions []seams.KeyAction) collection {
	return collection{title: "Key", noun: "key", nameWord: "action", valueWord: "key", actions: actions}
}

// toggled turns a setting that is on or off in place, on space.
func (f settingsForm) toggled(keys keyMap, msg tea.KeyPressMsg, field setting) settingsForm {
	if !key.Matches(msg, keys.toggleOption) {
		return f
	}

	on, _ := f.value(field.path).(bool)

	// Laid out again, since a hint can name a setting turned on or off.
	return f.with(field.path, !on).laidOut()
}

// chosen moves a choice on in place, on →, or back, on ←.
func (f settingsForm) chosen(keys keyMap, msg tea.KeyPressMsg, field setting) settingsForm {
	switch {
	case key.Matches(msg, keys.cycleRight):
		return f.with(field.path, field.cycled(f.text(field), 1))
	case key.Matches(msg, keys.cycleLeft):
		return f.with(field.path, field.cycled(f.text(field), -1))
	default:
		return f
	}
}

// text is a setting's value as text.
func (f settingsForm) text(field setting) string {
	text, _ := f.value(field.path).(string)

	return text
}

// with is the form with a setting's value edited, and every refusal of it
// still standing, a write's or Settings' own check's, taken away.
func (f settingsForm) with(path string, value any) settingsForm {
	f.edits = maps.Clone(f.edits)
	f.edits[path] = value
	f.refusals = f.refusals.without(path)

	return f
}
