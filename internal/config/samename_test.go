// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestTwoNamesThatAreOneAreRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, file string
		named      []string
	}{
		{
			name:  "jira headers differing in case",
			file:  `{"jira": {"headers": {"CF-Access-Client-Secret": "one-1111", "cf-access-client-secret": "two-2222"}}}`,
			named: []string{`"CF-Access-Client-Secret"`, `"cf-access-client-secret"`},
		},
		{
			name:  "branch prefixes differing in case",
			file:  `{"branch": {"prefixes": {"Spike": "research", "spike": "explore"}}}`,
			named: []string{`"Spike"`, `"spike"`},
		},
		{
			name:  "branch prefixes differing in surrounding space",
			file:  `{"branch": {"prefixes": {"Bug": "fix", " bug ": "bugfix"}}}`,
			named: []string{`"Bug"`, `" bug "`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.Parse(strings.NewReader(test.file))

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrSameName) {
				t.Fatalf("Parse returned %v, want ErrSameName", err)
			}

			for _, named := range test.named {
				if !strings.Contains(err.Error(), named) {
					t.Errorf("the refusal %q does not name %s", err, named)
				}
			}
		})
	}
}

func TestNamesThatDifferBeyondCaseAreTaken(t *testing.T) {
	t.Parallel()

	// Arrange
	file := `{"jira": {"headers": {"X-Proxy-Token": "one-1111", "X-Proxy-User": "two-2222"}},
		"branch": {"prefixes": {"Bug": "fix", "Spike": "research"}}}`

	// Act
	cfg, err := config.Parse(strings.NewReader(file))

	// Assert
	if err != nil || len(cfg.Jira.Headers) != 2 || len(cfg.Branch.Prefixes) != 2 {
		t.Errorf("Parse returned %d headers, %d prefixes, %v; want both of each taken",
			len(cfg.Jira.Headers), len(cfg.Branch.Prefixes), err)
	}
}
