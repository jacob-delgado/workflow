// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestLoadAcceptsTheCurrentVersionAndRejectsAnUnknownOne(t *testing.T) {
	t.Parallel()

	// Arrange
	accepted := t.TempDir()
	write(t, accepted, `{"version": "`+config.CurrentVersion+`"}`)

	rejected := t.TempDir()
	write(t, rejected, `{"version": "99"}`)

	// Act
	current, currentErr := config.Load(accepted, t.TempDir())
	_, unknownErr := config.Load(rejected, t.TempDir())

	// Assert
	if currentErr != nil {
		t.Errorf("Load with the current version returned %v, want nil", currentErr)
	}

	if current.Version != config.CurrentVersion {
		t.Errorf("Version = %q, want the current version", current.Version)
	}

	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "99") {
		t.Errorf("Load with version 99 returned %v, want an error naming the version", unknownErr)
	}
}
