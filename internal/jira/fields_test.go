// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// resolveBody is a transition with a screen, shaped as Data Center documents
// expand=transitions.fields: the map is keyed by field id, and each field says
// whether it is required, whether it has a default, its schema, and — for a
// field with a fixed set of values — those values. A resolution names each value
// with "name"; a custom select list names it with "value".
const resolveBody = `{"transitions":[{"id":"5","name":"Resolve Issue",` +
	`"to":{"name":"Resolved","statusCategory":{"key":"done"}},"fields":{` +
	`"resolution":{"required":true,"hasDefaultValue":false,"name":"Resolution",` +
	`"schema":{"type":"resolution","system":"resolution"},` +
	`"allowedValues":[{"id":"1","name":"Fixed"},{"id":"2","name":"Won't Fix"}]},` +
	`"customfield_10200":{"required":true,"hasDefaultValue":false,"name":"Root cause",` +
	`"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea"}},` +
	`"fixVersions":{"required":true,"hasDefaultValue":false,"name":"Fix Version/s",` +
	`"schema":{"type":"array","items":"version","system":"fixVersions"},` +
	`"allowedValues":[{"id":"10000","name":"1.0"}]},` +
	`"customfield_10300":{"required":true,"hasDefaultValue":false,"name":"Team",` +
	`"schema":{"type":"option","custom":"select"},"allowedValues":[{"id":"7","value":"Platform"}]},` +
	`"assignee":{"required":true,"hasDefaultValue":false,"name":"Assignee",` +
	`"schema":{"type":"user","system":"assignee"}},` +
	`"duedate":{"required":true,"hasDefaultValue":false,"name":"Due date",` +
	`"schema":{"type":"date","system":"duedate"}},` +
	`"comment":{"required":false,"hasDefaultValue":false,"name":"Comment","schema":{"type":"comment"}},` +
	`"priority":{"required":true,"hasDefaultValue":true,"name":"Priority","schema":{"type":"priority"},` +
	`"allowedValues":[{"id":"3","name":"Major"}]}` +
	`}}]}`

func TestTransitionsListTheFieldsAMoveNeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	var query atomic.Value

	client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
		query.Store(request.URL.RawQuery)
		answer(resolveBody, "fred")(writer, request)
	})

	// Act
	found, err := client.Transitions(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	// Assert
	if got, _ := query.Load().(string); !strings.Contains(got, "expand=transitions.fields") {
		t.Errorf("query = %q, want the fields expanded", got)
	}

	if len(found) != 1 {
		t.Fatalf("got %d transitions, want the one in the fixture", len(found))
	}

	// Only what Jira will refuse the move without: required and with no
	// default. Ordered by name, so the same screen always asks in one order.
	want := []jira.Field{
		{ID: "assignee", Name: "Assignee", Kind: jira.FieldUser, Options: nil},
		{ID: "duedate", Name: "Due date", Kind: jira.FieldDate, Options: nil},
		{
			ID: "fixVersions", Name: "Fix Version/s", Kind: jira.FieldOptionList,
			Options: []jira.Option{{ID: "10000", Name: "1.0"}},
		},
		{
			ID: "resolution", Name: "Resolution", Kind: jira.FieldOption,
			Options: []jira.Option{{ID: "1", Name: "Fixed"}, {ID: "2", Name: "Won't Fix"}},
		},
		{ID: "customfield_10200", Name: "Root cause", Kind: jira.FieldText, Options: nil},
		{ID: "customfield_10300", Name: "Team", Kind: jira.FieldOption, Options: []jira.Option{{ID: "7", Name: "Platform"}}},
	}

	if got := found[0].Fields; !slices.EqualFunc(got, want, equalFields) {
		t.Errorf("Fields = %+v\nwant   %+v", got, want)
	}
}

func TestACascadingSelectIsRefusedRatherThanSentAParentOnly(t *testing.T) {
	t.Parallel()

	// Arrange
	// A cascading select carries allowedValues — its parents, each with a nested
	// child — so it must be told apart from a plain option list by its schema
	// type. Sent as one option it would carry a parent with no child, and Jira
	// would reject the whole transition.
	const body = `{"transitions":[{"id":"7","name":"Triage",` +
		`"to":{"name":"Triaged","statusCategory":{"key":"indeterminate"}},"fields":{` +
		`"customfield_10400":{"required":true,"hasDefaultValue":false,"name":"Category",` +
		`"schema":{"type":"option-with-child","custom":"cascadingselect"},` +
		`"allowedValues":[{"id":"10","value":"Parent","children":[{"id":"11","value":"Child"}]}]}` +
		`}}]}`

	client := serve(t, answer(body, "fred"))

	// Act
	found, err := client.Transitions(t.Context(), "OPS-1")
	if err != nil {
		t.Fatalf("Transitions returned %v, want nil", err)
	}

	// Assert
	if len(found) != 1 || len(found[0].Fields) != 1 {
		t.Fatalf("got %d transitions, want one with one field", len(found))
	}

	if kind := found[0].Fields[0].Kind; kind != jira.FieldUnsupported {
		t.Errorf("cascading select Kind = %d, want FieldUnsupported (%d)", kind, jira.FieldUnsupported)
	}
}

// equalFields compares two fields, options included.
func equalFields(got, want jira.Field) bool {
	return got.ID == want.ID && got.Name == want.Name && got.Kind == want.Kind && slices.Equal(got.Options, want.Options)
}

func TestAFieldCanSayWhetherItCanBeFilledHere(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind jira.FieldKind
		want bool
	}{
		"one of a set of values":  {kind: jira.FieldOption, want: true},
		"a list of set values":    {kind: jira.FieldOptionList, want: true},
		"text":                    {kind: jira.FieldText, want: true},
		"a user":                  {kind: jira.FieldUser, want: true},
		"a date":                  {kind: jira.FieldDate, want: true},
		"any other kind of field": {kind: jira.FieldUnsupported, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			field := jira.Field{ID: "x", Name: "X", Kind: tt.kind, Options: nil}

			// Act & Assert
			if got := field.Fillable(); got != tt.want {
				t.Errorf("Fillable() for kind %d = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// field is a field of a kind, named for its id.
func field(id string, kind jira.FieldKind) jira.Field {
	return jira.Field{ID: id, Name: id, Kind: kind, Options: nil}
}

func TestApplyTransitionSendsTheFieldValuesInTheShapeEachNeeds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		values  []jira.FieldValue
		present bool
		want    string
	}{
		"no values sends no fields key at all": {values: nil, present: false, want: ""},
		"an option, a list of options and text": {
			values: []jira.FieldValue{
				{Field: field("resolution", jira.FieldOption), OptionID: "1", Text: ""},
				{Field: field("fixVersions", jira.FieldOptionList), OptionIDs: []string{"10000"}, Text: ""},
				{Field: field("customfield_10200", jira.FieldText), OptionID: "", Text: "a \"nil\" token"},
			},
			present: true,
			want:    `{"customfield_10200":"a \"nil\" token","fixVersions":[{"id":"10000"}],"resolution":{"id":"1"}}`,
		},
		"a user, a date and several versions": {
			values: []jira.FieldValue{
				{Field: field("assignee", jira.FieldUser), Text: "fred"},
				{Field: field("duedate", jira.FieldDate), Text: "2026-09-21"},
				{Field: field("fixVersions", jira.FieldOptionList), OptionIDs: []string{"10000", "10001"}},
			},
			present: true,
			want:    `{"assignee":{"name":"fred"},"duedate":"2026-09-21","fixVersions":[{"id":"10000"},{"id":"10001"}]}`,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var (
				sent    atomic.Value
				present atomic.Bool
			)

			client := serve(t, func(writer http.ResponseWriter, request *http.Request) {
				var body map[string]json.RawMessage

				_ = json.NewDecoder(request.Body).Decode(&body)
				fields, found := body["fields"]
				sent.Store(string(fields))
				present.Store(found)
				writer.WriteHeader(http.StatusNoContent)
			})

			// Act
			err := client.ApplyTransition(t.Context(), "OPS-1", startProgress(), tt.values)

			// Assert
			if err != nil || present.Load() != tt.present || sent.Load() != tt.want {
				t.Errorf("ApplyTransition = %v; fields present %v, sent %v; want present %v, %s",
					err, present.Load(), sent.Load(), tt.present, tt.want)
			}
		})
	}
}
