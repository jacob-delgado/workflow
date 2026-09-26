// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// usedScope is the scope a commit used, as its subject line writes it.
const usedScope = "cli"

func TestRememberScopeRecordsTheScopeAsTheSubjectWritesIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		scope        string
		wantRecorded []string
	}{
		"a scope":                 {scope: usedScope, wantRecorded: []string{usedScope}},
		"a padded scope, trimmed": {scope: " " + usedScope + " ", wantRecorded: []string{usedScope}},
		"no scope":                {scope: "", wantRecorded: nil},
		"a whitespace-only scope": {scope: " ", wantRecorded: nil},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var recorded []string

			record := func(scope string) { recorded = append(recorded, scope) }

			// Act
			reported := loop.RememberScope(record, tt.scope)

			// Assert
			if !slices.Equal(recorded, tt.wantRecorded) || reported != (tt.wantRecorded != nil) {
				t.Errorf("RememberScope(%q) recorded %q and reported %t, want %q", tt.scope, recorded, reported,
					tt.wantRecorded)
			}
		})
	}
}

func TestRememberScopeWithNothingToRecordIntoReportsNothingRecorded(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if loop.RememberScope(nil, usedScope) {
		t.Error("RememberScope with no store reported the scope recorded")
	}
}
