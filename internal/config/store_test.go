// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestTheStoreIsOnByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Store.Disabled {
		t.Error("the store is disabled by default, want it on")
	}
}

func TestTheStoreCanBeDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"store": {"disabled": true}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if !cfg.Store.Disabled {
		t.Error("store.disabled was not read from the configuration")
	}
}
