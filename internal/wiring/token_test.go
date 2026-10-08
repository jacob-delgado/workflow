// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestResolveTokenPrefersTheFileThenTheCommand(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		literal, command string
		wantToken        string
		wantSource       string
	}{
		"the file wins": {
			literal: "file-token", command: "echo command-token",
			wantToken: "file-token", wantSource: "the configuration file",
		},
		"a command when the file is empty": {
			command:   "echo command-token",
			wantToken: "command-token", wantSource: commandSource,
		},
		"nothing configured": {
			wantToken: "", wantSource: "no token configured",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			token, source, err := wiring.ResolveToken(t.Context(), config.Secret(tt.literal), tt.command, "")
			if err != nil {
				t.Fatalf("ResolveToken returned %v, want nil", err)
			}

			// Assert
			if token.Reveal() != tt.wantToken || source != tt.wantSource {
				t.Errorf("ResolveToken = %q from %q, want %q from %q", token, source, tt.wantToken, tt.wantSource)
			}
		})
	}
}

func TestResolveTokenReadsAnEnvironmentVariable(t *testing.T) {
	// Not parallel: t.Setenv forbids it.
	// Arrange
	t.Setenv("WF_TEST_TOKEN", "env-token")

	// Act
	token, source, err := wiring.ResolveToken(t.Context(), "", "", "WF_TEST_TOKEN")
	if err != nil {
		t.Fatalf("ResolveToken returned %v, want nil", err)
	}

	// Assert
	if token != "env-token" || !strings.Contains(source, "WF_TEST_TOKEN") {
		t.Errorf("ResolveToken = %q from %q, want the env value and its name in the source", token, source)
	}
}

func TestResolveTokenReportsAFailedCommand(t *testing.T) {
	t.Parallel()

	// Act
	_, _, err := wiring.ResolveToken(t.Context(), "", failingCommand, "")

	// Assert
	if err == nil {
		t.Error("ResolveToken(false) = nil error, want the command's failure reported")
	}
}

func TestAFailedTokenCommandIsToldWithoutWhatItPrinted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A command that reads a secret can print it, or what leads to it, as it
	// fails, and the error reaches doctor, the terminal and the request log.
	command := writeTokenCommand(t, "printf 'SECRET-VALUE at secret/path\\n' >&2\nexit 1\n")

	// Act
	_, _, err := wiring.ResolveToken(t.Context(), "", command.command, "")

	// Assert
	if err == nil || strings.Contains(err.Error(), "SECRET-VALUE") || strings.Contains(err.Error(), "secret/path") {
		t.Errorf("ResolveToken = %v, want the failure told without what the command printed", err)
	}
}

func TestATokenCommandNotOnThePathSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := filepath.Join(t.TempDir(), "no-such-program")

	// Act
	_, _, err := wiring.ResolveToken(t.Context(), "", missing, "")

	// Assert
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("ResolveToken = %v, want proc.ErrNotFound", err)
	}
}
