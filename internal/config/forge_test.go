// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestForgeCLIIsRead(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"forge": {"cli": true}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if !cfg.Forge.CLI {
		t.Errorf("forge.cli = %v, want it read as true", cfg.Forge.CLI)
	}
}
