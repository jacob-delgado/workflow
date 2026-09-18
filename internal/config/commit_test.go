// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestCommitSectionIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"commit": {"default_scope": "api"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Commit.DefaultScope != "api" {
		t.Errorf("commit = %+v, want the configured default scope", cfg.Commit)
	}
}

func TestADefaultScopeWithBadCharactersIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"commit": {"default_scope": "Two Words"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidCommit) {
		t.Errorf("Load returned %v, want it refused for an ill-formed default scope", err)
	}
}
