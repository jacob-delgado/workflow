// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// Fields a transition can ask for, one of each kind the checks tell apart.
func resolutionField() jira.Field {
	return jira.Field{
		ID: "resolution", Name: "Resolution", Kind: jira.FieldOption,
		Options: []jira.Option{{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"}},
	}
}

func versionsField() jira.Field {
	return jira.Field{
		ID: "fixVersions", Name: "Fix Version/s", Kind: jira.FieldOptionList,
		Options: []jira.Option{{ID: "10", Name: "1.0"}, {ID: "11", Name: "1.1"}},
	}
}

func dueField() jira.Field { return jira.Field{ID: "due", Name: dueDate, Kind: jira.FieldDate} }

// dueDate is the due date field's name.
const dueDate = "Due date"

func causeField() jira.Field {
	return jira.Field{ID: "customfield_1", Name: "Root cause", Kind: jira.FieldText}
}

func cascadeField() jira.Field {
	return jira.Field{ID: "customfield_2", Name: "Component tree", Kind: jira.FieldUnsupported}
}

func reviewerField() jira.Field {
	return jira.Field{ID: "customfield_3", Name: "Reviewer", Kind: jira.FieldUser}
}

func TestAFieldValueIsCheckedAgainstItsField(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		value jira.FieldValue
		want  error
	}{
		"an allowed option": {value: jira.FieldValue{Field: resolutionField(), OptionID: "2"}, want: nil},
		"no option":         {value: jira.FieldValue{Field: resolutionField()}, want: jira.ErrNeedsValue},
		"an option it does not have": {
			value: jira.FieldValue{Field: resolutionField(), OptionID: "9"}, want: jira.ErrNotAnOption,
		},
		"allowed options": {value: jira.FieldValue{Field: versionsField(), OptionIDs: []string{"10", "11"}}, want: nil},
		"no options":      {value: jira.FieldValue{Field: versionsField()}, want: jira.ErrNeedsChoice},
		"one option it does not have": {
			value: jira.FieldValue{Field: versionsField(), OptionIDs: []string{"10", "99"}}, want: jira.ErrNotAnOption,
		},
		"a date":            {value: jira.FieldValue{Field: dueField(), Text: "2026-09-21"}, want: nil},
		"not a date":        {value: jira.FieldValue{Field: dueField(), Text: "next week"}, want: jira.ErrNeedsDate},
		"text":              {value: jira.FieldValue{Field: causeField(), Text: "a race"}, want: nil},
		"blank text":        {value: jira.FieldValue{Field: causeField(), Text: "   "}, want: jira.ErrNeedsValue},
		"a username":        {value: jira.FieldValue{Field: reviewerField(), Text: "ana"}, want: nil},
		"no username":       {value: jira.FieldValue{Field: reviewerField()}, want: jira.ErrNeedsValue},
		"a field only Jira": {value: jira.FieldValue{Field: cascadeField(), Text: "x"}, want: jira.ErrOnlyJira},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := tt.value.Check()

			// Assert
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("Check() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestATransitionNamesTheFirstFieldOnlyJiraCanFill(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		fields    []jira.Field
		wantField string
		wantFound bool
	}{
		"no fields":       {fields: nil, wantField: "", wantFound: false},
		"fillable fields": {fields: []jira.Field{resolutionField(), dueField()}, wantField: "", wantFound: false},
		"one only Jira fills": {
			fields: []jira.Field{resolutionField(), cascadeField()}, wantField: cascadeField().ID, wantFound: true,
		},
		"the first one is it": {
			fields: []jira.Field{cascadeField(), causeField()}, wantField: cascadeField().ID, wantFound: true,
		},
		"every kind it fills": {
			fields: []jira.Field{causeField(), reviewerField(), versionsField()}, wantField: "", wantFound: false,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			move := jira.Transition{ID: "5", Name: "Resolve", ToStatus: "Resolved", Fields: tt.fields}

			// Act
			field, found := move.Unfillable()

			// Assert
			if found != tt.wantFound || field.ID != tt.wantField {
				t.Errorf("Unfillable() = %q, %v; want %q, %v", field.ID, found, tt.wantField, tt.wantFound)
			}
		})
	}
}

func TestATransitionFindsItsFieldByID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		id        string
		wantName  string
		wantFound bool
	}{
		"a field it asks for":         {id: "due", wantName: dueDate, wantFound: true},
		"a field it does not ask for": {id: "summary", wantName: "", wantFound: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			move := jira.Transition{ID: "5", Fields: []jira.Field{resolutionField(), dueField()}}

			// Act
			field, found := move.Field(tt.id)

			// Assert
			if found != tt.wantFound || field.Name != tt.wantName {
				t.Errorf("Field(%q) = %+v, %v; want %q, %v", tt.id, field, found, tt.wantName, tt.wantFound)
			}
		})
	}
}
