// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestTaskwarriorIsOnAndUnconfiguredByDefault(t *testing.T) {
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
	if cfg.Taskwarrior.Program != "" || cfg.Taskwarrior.Disabled {
		t.Errorf("taskwarrior = %+v, want no program and not disabled", cfg.Taskwarrior)
	}
}

func TestTaskwarriorProgramAndDisabledAreRead(t *testing.T) {
	t.Parallel()

	// Arrange
	homeDir := t.TempDir()
	write(t, homeDir, `{"taskwarrior": {"program": "/opt/homebrew/bin/task", "disabled": true}}`)

	// Act
	cfg, err := config.Load(t.TempDir(), homeDir)
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Taskwarrior.Program != "/opt/homebrew/bin/task" || !cfg.Taskwarrior.Disabled {
		t.Errorf("taskwarrior = %+v, want the program and disabled read from the file", cfg.Taskwarrior)
	}
}

func TestATaskwarriorProgramThatIsNoNameOrAbsolutePathIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a line feed":       `a\nb`,
		"a carriage return": `a\rb`,
		"a NUL":             `a\u0000b`,
		"a relative path":   `bin/task`,
		"a leading space":   ` /opt/task`,
	}

	for name, program := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			homeDir := t.TempDir()
			write(t, homeDir, `{"taskwarrior": {"program": "`+program+`"}}`)

			// Act
			_, err := config.Load(t.TempDir(), homeDir)

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidTaskwarriorProgram) {
				t.Errorf("Load returned %v, want it refused as an invalid taskwarrior.program", err)
			}
		})
	}
}

func TestATaskwarriorProgramIsKeptAsWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	homeDir := t.TempDir()
	write(t, homeDir, `{"taskwarrior": {"program": "/opt/my task "}}`)

	// Act
	cfg, err := config.Load(t.TempDir(), homeDir)
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.Taskwarrior.Program != "/opt/my task " {
		t.Errorf("taskwarrior.program = %q, want it kept as written, spaces and all", cfg.Taskwarrior.Program)
	}
}
