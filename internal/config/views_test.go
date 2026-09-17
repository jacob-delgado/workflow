// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestConfiguredViewsAreRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"jira": {"base_url": "https://jira.example.com", "views": [`+
		`{"name": "My work", "jql": "assignee = currentUser()"},`+
		`{"name": "Sprint", "jql": "sprint in openSprints()"}]}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	views := cfg.Jira.Views
	if len(views) != 2 || views[0].Name != "My work" || views[1].JQL != "sprint in openSprints()" {
		t.Errorf("views = %+v, want the two configured views", views)
	}
}

func TestAViewMissingItsNameOrQueryIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"no jql":  `{"jira": {"base_url": "https://x", "views": [{"name": "Sprint"}]}}`,
		"no name": `{"jira": {"base_url": "https://x", "views": [{"jql": "project = OPS"}]}}`,
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, contents)

			// Act
			_, err := config.Load(workDir, t.TempDir())

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidView) {
				t.Errorf("Load = %v, want a half-written view refused", err)
			}
		})
	}
}
