// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"
)

// Why a value cannot be sent for the field it fills. Each reads after the
// field's name: "Resolution needs a value".
var (
	// ErrNeedsValue is a field left empty.
	ErrNeedsValue = errors.New("needs a value")
	// ErrNeedsDate is a date field holding something that is not a date.
	ErrNeedsDate = errors.New("must be a date written " + DateShape)
	// ErrNeedsChoice is a list field with nothing chosen.
	ErrNeedsChoice = errors.New("needs at least one")
	// ErrNotAnOption is a choice the field does not allow.
	ErrNotAnOption = errors.New("is not one of its values")
	// ErrOnlyJira is a field only Jira's own screen can fill, such as a
	// cascading select. A transition is never sent half-filled, to be refused.
	ErrOnlyJira = errors.New("which only Jira's own screen can fill; make this change in Jira")
)

// DateLayout is the calendar date Jira reads and writes, spelled in Go's own
// reference date.
const DateLayout = "2006-01-02"

// DateShape is DateLayout as a person reads it: the hint a date field shows and
// the shape its refusal names, so neither reads as a stale example date.
const DateShape = "YYYY-MM-DD"

// FieldKind is how a field a transition needs can be filled in.
type FieldKind int

const (
	// FieldUnsupported is a field only Jira's own screens can fill, such as a
	// cascading select, or a type this does not recognize.
	FieldUnsupported FieldKind = iota
	// FieldOption takes one of a fixed set of values, such as a resolution.
	FieldOption
	// FieldOptionList takes any number of them, such as fix versions.
	FieldOptionList
	// FieldText takes free text.
	FieldText
	// FieldUser takes a username, such as an assignee.
	FieldUser
	// FieldDate takes a calendar date.
	FieldDate
)

// Option is one value a field allows.
type Option struct {
	ID   string
	Name string
}

// Field is one field a transition cannot be made without.
type Field struct {
	ID      string
	Name    string
	Kind    FieldKind
	Options []Option
}

// Fillable reports whether the field can be filled in here rather than only in
// Jira.
func (f Field) Fillable() bool {
	return f.Kind != FieldUnsupported
}

// Unfillable is the first field of the transition only Jira's own screen can
// fill, if it has one: the transition cannot be made from here.
func (t Transition) Unfillable() (Field, bool) {
	for _, field := range t.Fields {
		if !field.Fillable() {
			return field, true
		}
	}

	return Field{}, false
}

// Field is the field of this id the transition asks for, if it asks for one.
func (t Transition) Field(fieldID string) (Field, bool) {
	index := slices.IndexFunc(t.Fields, func(field Field) bool { return field.ID == fieldID })
	if index < 0 {
		return Field{}, false
	}

	return t.Fields[index], true
}

// allows reports whether the field offers an option of this id.
func (f Field) allows(optionID string) bool {
	return slices.ContainsFunc(f.Options, func(option Option) bool { return option.ID == optionID })
}

// FieldValue is what was chosen or typed for a field.
type FieldValue struct {
	Field Field
	// OptionID is the chosen option, for a single-option field.
	OptionID string
	// OptionIDs are the chosen options, for a list field.
	OptionIDs []string
	// Text is what was typed, for a text, user or date field.
	Text string
}

// Check says why the value cannot be sent for its field — empty, not a date,
// not one of the field's options, or a field only Jira can fill — or nil when
// it can. Typed text counts as empty when it is only spaces. A map, not a
// switch, as payload's is.
func (v FieldValue) Check() error {
	return map[FieldKind]func() error{
		FieldUnsupported: func() error { return ErrOnlyJira },
		FieldOption:      v.checkOption,
		FieldOptionList:  v.checkOptions,
		FieldText:        v.checkText,
		FieldUser:        v.checkText,
		FieldDate:        v.checkText,
	}[v.Field.Kind]()
}

// wireField is a field as expand=transitions.fields describes it.
type wireField struct {
	Required bool   `json:"required"`
	Default  bool   `json:"hasDefaultValue"`
	Name     string `json:"name"`
	Schema   struct {
		Type string `json:"type"`
	} `json:"schema"`
	// Each allowed value names itself with "name" or, on a custom select list,
	// with "value".
	AllowedValues []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"allowedValues"`
}

// requiredFields lists the fields Jira will refuse a transition without: those
// required and with no default. They are ordered by name, so the same screen
// always asks in the same order — a map has none of its own.
func requiredFields(wire map[string]wireField) []Field {
	fields := make([]Field, 0, len(wire))

	for fieldID, each := range wire {
		if each.Required && !each.Default {
			fields = append(fields, each.field(fieldID))
		}
	}

	slices.SortFunc(fields, func(left, right Field) int {
		return cmp.Or(cmp.Compare(left.Name, right.Name), cmp.Compare(left.ID, right.ID))
	})

	return fields
}

// field flattens a wire field.
func (w wireField) field(fieldID string) Field {
	options := make([]Option, 0, len(w.AllowedValues))

	for _, value := range w.AllowedValues {
		options = append(options, Option{ID: value.ID, Name: cmp.Or(value.Name, value.Value)})
	}

	return Field{ID: fieldID, Name: w.Name, Kind: w.kind(), Options: options}
}

// kind is how the field can be filled: from its allowed values if it lists any,
// as text if its schema is a string, and otherwise not here at all.
func (w wireField) kind() FieldKind {
	switch {
	case w.Schema.Type == "option-with-child":
		// A cascading select lists its parents in allowedValues, so it would
		// otherwise pass for a plain option; sent as one it drops the child Jira
		// requires. Only Jira's own screen fills it.
		return FieldUnsupported
	case len(w.AllowedValues) > 0 && w.Schema.Type == "array":
		return FieldOptionList
	case len(w.AllowedValues) > 0:
		return FieldOption
	case w.Schema.Type == "user":
		return FieldUser
	case w.Schema.Type == "date":
		return FieldDate
	case w.Schema.Type == "string":
		return FieldText
	default:
		return FieldUnsupported
	}
}

// reference names an option, or a transition, by id on the wire.
type reference struct {
	ID string `json:"id"`
}

// userRef names a user by username, the way Jira Data Center identifies one in
// a field value. Cloud keys a user by account id instead; this does not target
// it.
type userRef struct {
	Name string `json:"name"`
}

// payload is the value in the shape Jira reads for its kind of field. It is
// any because the shapes — an object, a list of them, a user, a string — share
// nothing but being encodable. A map, not a switch, so there is no last-case
// arm gobco can never see; exhaustive keeps it complete.
func (v FieldValue) payload() any {
	return map[FieldKind]any{
		FieldOptionList:  v.optionRefs(),
		FieldOption:      reference{ID: v.OptionID},
		FieldUser:        userRef{Name: v.Text},
		FieldUnsupported: v.Text,
		FieldText:        v.Text,
		FieldDate:        v.Text,
	}[v.Field.Kind]
}

// optionRefs references each chosen option by id.
func (v FieldValue) optionRefs() []reference {
	refs := make([]reference, 0, len(v.OptionIDs))
	for _, id := range v.OptionIDs {
		refs = append(refs, reference{ID: id})
	}

	return refs
}

// fieldsPayload is the fields object of a transition, or nil for none, which
// leaves the key out of the request entirely.
func fieldsPayload(values []FieldValue) map[string]any {
	if len(values) == 0 {
		return nil
	}

	fields := make(map[string]any, len(values))
	for _, value := range values {
		fields[value.Field.ID] = value.payload()
	}

	return fields
}

// checkOption refuses no option chosen, or one the field does not offer.
func (v FieldValue) checkOption() error {
	if v.OptionID == "" {
		return ErrNeedsValue
	}

	if !v.Field.allows(v.OptionID) {
		return ErrNotAnOption
	}

	return nil
}

// checkOptions refuses nothing chosen, or any choice the field does not offer.
func (v FieldValue) checkOptions() error {
	if len(v.OptionIDs) == 0 {
		return ErrNeedsChoice
	}

	for _, id := range v.OptionIDs {
		if !v.Field.allows(id) {
			return ErrNotAnOption
		}
	}

	return nil
}

// checkText refuses blank text, and a date field's text that is not a date.
func (v FieldValue) checkText() error {
	text := strings.TrimSpace(v.Text)
	if text == "" {
		return ErrNeedsValue
	}

	if v.Field.Kind != FieldDate {
		return nil
	}

	_, err := time.Parse(DateLayout, text)
	if err != nil {
		return ErrNeedsDate
	}

	return nil
}
