// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
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

			record := func(scope string) error {
				recorded = append(recorded, scope)

				return nil
			}

			// Act
			reported, err := loop.RememberScope(record, tt.scope)

			// Assert
			if err != nil || !slices.Equal(recorded, tt.wantRecorded) || reported != (tt.wantRecorded != nil) {
				t.Errorf("RememberScope(%q) recorded %q and reported %t, %v; want %q", tt.scope, recorded, reported,
					err, tt.wantRecorded)
			}
		})
	}
}

func TestRememberScopeWithNothingToRecordIntoReportsNothingRecorded(t *testing.T) {
	t.Parallel()

	// Act
	reported, err := loop.RememberScope(nil, usedScope)

	// Assert
	if reported || err != nil {
		t.Errorf("RememberScope with no store = %t, %v; want nothing recorded", reported, err)
	}
}

func TestRememberScopeSaysWhyARecordWasNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	record := func(string) error { return errSeam }

	// Act
	reported, err := loop.RememberScope(record, usedScope)

	// Assert
	if !reported || !errors.Is(err, errSeam) {
		t.Errorf("RememberScope = %t, %v; want the scope handed over and why it was not kept", reported, err)
	}
}
