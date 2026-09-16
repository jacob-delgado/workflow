// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"cmp"
	"slices"
)

// FieldKind is how a field a transition needs can be filled in.
type FieldKind int

const (
	// FieldUnsupported is a field only Jira's own screens can fill: a user
	// picker, a date, a cascading select.
	FieldUnsupported FieldKind = iota
	// FieldOption takes one of a fixed set of values, such as a resolution.
	FieldOption
	// FieldOptionList takes a list of them, such as fix versions; one is
	// chosen here.
	FieldOptionList
	// FieldText takes free text.
	FieldText
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

// FieldValue is what was chosen or typed for a field.
type FieldValue struct {
	Field Field
	// OptionID is the chosen option, for a field with options.
	OptionID string
	// Text is what was typed, for a text field.
	Text string
}

// wireField is a field as expand=transitions.fields describes it.
type wireField struct {
	Required bool   `json:"required"`
	Default  bool   `json:"hasDefaultValue"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
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
	} `json:"allowedValues"` //nolint:tagliatelle // Jira's field name on the wire, not ours to pick
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
	case len(w.AllowedValues) > 0 && w.Schema.Type == "array":
		return FieldOptionList
	case len(w.AllowedValues) > 0:
		return FieldOption
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

// payload is the value in the shape Jira reads for its kind of field. It is
// any because the three shapes — an object, a list of them, a string — share
// nothing but being encodable.
func (v FieldValue) payload() any {
	if v.Field.Kind == FieldOptionList {
		return []reference{{ID: v.OptionID}}
	}

	if v.Field.Kind == FieldOption {
		return reference{ID: v.OptionID}
	}

	return v.Text
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
