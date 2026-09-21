// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

func TestScopesReadsTheScopesOutOfSubjects(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		subjects []string
		want     []string
	}{
		"a scope is read": {
			subjects: []string{"feat(parser): rewrite the lexer"},
			want:     []string{"parser"},
		},
		"a breaking scope is read": {
			subjects: []string{"feat(api)!: drop a field"},
			want:     []string{"api"},
		},
		"a scopeless subject contributes nothing": {
			subjects: []string{"fix: a typo", "docs: a note"},
			want:     nil,
		},
		"scopes keep first-seen order and appear once": {
			subjects: []string{"feat(tui): a", "fix(jira): b", "test(tui): c"},
			want:     []string{"tui", "jira"},
		},
		"a scope with characters a scope may not hold is dropped": {
			subjects: []string{"feat(TUI): a", "fix(a b): b", "chore(deps): c"},
			want:     []string{"deps"},
		},
		"whitespace around a scope is trimmed off": {
			subjects: []string{"feat( tui ): a", "fix(tui): b"},
			want:     []string{"tui"},
		},
		"a subject that is not a Conventional Commit is skipped": {
			subjects: []string{"Merge branch 'main'", "WIP", "feat(): empty"},
			want:     nil,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := convention.Scopes(testCase.subjects)

			// Assert
			if !slices.Equal(got, testCase.want) {
				t.Errorf("Scopes(%q) = %q, want %q", testCase.subjects, got, testCase.want)
			}
		})
	}
}
