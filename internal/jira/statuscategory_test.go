// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// categoryKeys is each key a status's category arrives as, and the category it
// is read as: Jira's three, its No Category ("undefined"), a key this build
// does not know, and none at all.
func categoryKeys() map[string]jira.StatusCategory {
	return map[string]jira.StatusCategory{
		"new":           jira.CategoryNew,
		"indeterminate": jira.CategoryIndeterminate,
		"done":          jira.CategoryDone,
		"undefined":     jira.CategoryUnknown,
		"invented":      jira.CategoryUnknown,
		"":              jira.CategoryUnknown,
	}
}

func TestASearchedIssueReadsItsStatusCategoryFromItsKey(t *testing.T) {
	t.Parallel()

	for key, want := range categoryKeys() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			body := `{"startAt":0,"maxResults":50,"total":1,"issues":[` +
				`{"key":"OPS-1","fields":{"summary":"Fix login",` +
				`"status":{"name":"Triage","statusCategory":{"key":"` + key + `"}}}}]}`
			client := serve(t, answer(body, "fred"))

			// Act
			result, err := client.Search(t.Context(), jira.AssignedToMe, 0)

			// Assert
			if err != nil || len(result.Issues) != 1 {
				t.Fatalf("Search = %+v, %v; want one issue", result, err)
			}

			if got := result.Issues[0].StatusCategory; got != want {
				t.Errorf("a status keyed %q reads as %q, want %q", key, got.Word(), want.Word())
			}
		})
	}
}

func TestATransitionReadsItsStatusCategoryFromItsKey(t *testing.T) {
	t.Parallel()

	for key, want := range categoryKeys() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			body := `{"expand":"transitions","transitions":[` +
				`{"id":"21","name":"Triage","to":{"name":"Triage","statusCategory":{"key":"` + key + `"}}}]}`
			client := serve(t, answer(body, "fred"))

			// Act
			found, err := client.Transitions(t.Context(), "OPS-1")

			// Assert
			if err != nil || len(found) != 1 {
				t.Fatalf("Transitions = %+v, %v; want one transition", found, err)
			}

			if got := found[0].ToStatusCategory; got != want {
				t.Errorf("a status keyed %q reads as %q, want %q", key, got.Word(), want.Word())
			}
		})
	}
}

func TestAStatusCategoryIsParsedFromItsKey(t *testing.T) {
	t.Parallel()

	for key, want := range categoryKeys() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := jira.ParseStatusCategory(key); got != want {
				t.Errorf("ParseStatusCategory(%q) = %q, want %q", key, got.Word(), want.Word())
			}
		})
	}
}

func TestAStatusCategoryIsWrittenAsJirasKey(t *testing.T) {
	t.Parallel()

	cases := map[jira.StatusCategory]string{
		jira.CategoryNew:           "new",
		jira.CategoryIndeterminate: "indeterminate",
		jira.CategoryDone:          "done",
		jira.CategoryUnknown:       "undefined",
	}

	for category, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act
			word := category.Word()

			// Assert
			if word != want {
				t.Errorf("Word() = %q, want %q", word, want)
			}

			if again := jira.ParseStatusCategory(word); again != category {
				t.Errorf("ParseStatusCategory(%q) = %q, want %q back", word, again.Word(), want)
			}
		})
	}
}
