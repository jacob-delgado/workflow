// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

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
		cfg.Branch.Prefixes["Bug"] != "bugfix" {
		t.Errorf("branch = %+v, want the configured template, prefix and map", cfg.Branch)
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
