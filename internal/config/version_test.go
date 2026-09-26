// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestLoadAcceptsTheCurrentVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"version": "`+config.CurrentVersion+`"}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	// Assert
	if err != nil {
		t.Fatalf("Load with the current version returned %v, want nil", err)
	}

	if cfg.Version != config.CurrentVersion {
		t.Errorf("Version = %q, want the current version", cfg.Version)
	}
}

func TestLoadRejectsAnUnknownVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"version": "99"}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrUnknownVersion) || !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Load with version 99 returned %v, want ErrUnknownVersion as an invalid file", err)
	}

	if err == nil || !strings.Contains(err.Error(), `"99"`) {
		t.Errorf("Load with version 99 returned %v, want an error naming the version", err)
	}
}
