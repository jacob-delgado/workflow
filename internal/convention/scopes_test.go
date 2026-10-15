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
		// A type may hold a digit after its first letter, as ValidateType
		// accepts one.
		"a scope after a type with a digit is read": {
			subjects: []string{"hotfix2(api): x"},
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

// The scopes the suggestions below are named.
const (
	apiScope      = "api"
	configScope   = "config"
	internalScope = "internal"
)

func TestScopeSuggestionsOffersTheStagedFilesDirectoryThenTheLogsScopes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		paths, subjects []string
		want            []string
	}{
		"files under one directory offer its name": {
			paths: []string{"internal/config/a.go", "internal/config/b.go"},
			want:  []string{configScope},
		},
		"files across sibling directories offer their parent": {
			paths: []string{"internal/tui/a.go", "internal/jira/b.go"},
			want:  []string{internalScope},
		},
		"a file above another's directory offers its own": {
			paths: []string{"internal/a.go", "internal/tui/b.go"},
			want:  []string{internalScope},
		},
		"a file below another's directory offers the other's": {
			paths: []string{"internal/tui/a.go", "internal/b.go"},
			want:  []string{internalScope},
		},
		"files at the repository root share no directory": {
			paths: []string{"main.go", "go.mod"},
			want:  nil,
		},
		"a file at the root of an absolute path shares no directory": {
			paths: []string{"/main.go"},
			want:  nil,
		},
		"a directory whose name is not a scope is not offered": {
			paths: []string{"my dir/a.go", "my dir/b.go"},
			want:  nil,
		},
		"with nothing staged the log's scopes alone are offered": {
			subjects: []string{"feat(api): add a field"},
			want:     []string{apiScope},
		},
		"the directory comes first and each scope once": {
			paths:    []string{"internal/jira/a.go"},
			subjects: []string{"fix(hooks): tidy", "feat(jira): add a field", "test(hooks): cover"},
			want:     []string{"jira", "hooks"},
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := convention.ScopeSuggestions(testCase.paths, testCase.subjects)

			// Assert
			if !slices.Equal(got, testCase.want) {
				t.Errorf("ScopeSuggestions(%q, %q) = %q, want %q", testCase.paths, testCase.subjects, got, testCase.want)
			}
		})
	}
}
