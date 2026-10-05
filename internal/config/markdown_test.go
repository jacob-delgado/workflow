// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestMarkdownCommentsAreOnUnlessAFileTurnsThemOff(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want bool
	}{
		"left out":       {body: `{"jira": {"base_url": "https://jira.example.com"}}`, want: true},
		"turned off":     {body: `{"jira": {"markdown_comments": false}}`, want: false},
		"turned on":      {body: `{"jira": {"markdown_comments": true}}`, want: true},
		"no jira at all": {body: `{}`, want: true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			cfg, err := config.Parse(strings.NewReader(testCase.body))
			// Assert
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if cfg.Jira.MarkdownComments != testCase.want {
				t.Errorf("MarkdownComments = %v, want %v", cfg.Jira.MarkdownComments, testCase.want)
			}
		})
	}
}

func TestConfigInitWritesMarkdownCommentsOn(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if !config.Template().Jira.MarkdownComments {
		t.Error("Template().Jira.MarkdownComments = false, want true")
	}
}
