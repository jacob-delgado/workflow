// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// bugType is the issue type the branch tests map to a prefix of its own.
const bugType = "Bug"

func TestBranchSectionIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"branch": {"template": "{prefix}/{key}", "default_prefix": "chore", `+
		`"prefixes": {"Bug": "bugfix"}}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Branch.Template != "{prefix}/{key}" || cfg.Branch.DefaultPrefix != "chore" ||
		cfg.Branch.Prefixes[bugType] != "bugfix" {
		t.Errorf("branch = %+v, want the configured template, prefix and map", cfg.Branch)
	}
}

func TestBranchNamingUsesTheSection(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		section   config.Branch
		issueType string
		want      string
	}{
		"an empty section keeps the built-in convention": {
			section: config.Branch{}, issueType: bugType, want: "fix/PROJ-7-redact-the-token",
		},
		"the template and a mapped prefix": {
			section: config.Branch{
				Template: "{prefix}/{key}", Prefixes: map[string]string{bugType: "bugfix"},
			},
			issueType: bugType, want: "bugfix/PROJ-7",
		},
		"the default prefix for a type not mapped": {
			section: config.Branch{DefaultPrefix: "chore"}, issueType: "Story", want: "chore/PROJ-7-redact-the-token",
		},
		"the slug limit": {
			section: config.Branch{SlugLimit: 6}, issueType: "Story", want: "feat/PROJ-7-redact",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.section.Naming().Name(tt.issueType, "PROJ-7", "Redact the token")

			// Assert
			if got != tt.want {
				t.Errorf("Naming().Name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestABranchTemplateWithoutTheKeyIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"branch": {"template": "{prefix}/{slug}"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidBranch) {
		t.Errorf("Load returned %v, want it refused for hiding the issue key", err)
	}
}

func TestANegativeSlugLimitIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"branch": {"slug_limit": -3}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidBranch) {
		t.Errorf("Load returned %v, want a negative slug limit refused", err)
	}
}
