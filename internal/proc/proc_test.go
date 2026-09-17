// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// missingProgram is a name no machine has on its PATH.
const missingProgram = "workflow-program-that-does-not-exist"

// goProgram is the one binary guaranteed present while `go test` is running,
// which is what lets these tests spawn a real process without depending on
// anything the repository does not already require.
const goProgram = "go"

func TestRunReturnsStandardOutput(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.Run(t.Context(), goProgram, "version")
	if err != nil {
		t.Fatalf("Run(go version) returned %v, want nil", err)
	}

	// Assert
	if !strings.Contains(string(output), "go version") {
		t.Errorf("Run(go version) = %q, want it to contain %q", output, "go version")
	}
}

func TestRunReportsAProgramNotOnPath(t *testing.T) {
	t.Parallel()

	// Act
	_, err := proc.Run(t.Context(), missingProgram)

	// Assert
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Run(%q) returned %v, want ErrNotFound", missingProgram, err)
	}
}

func TestRunReportsAFailureWithItsStandardError(t *testing.T) {
	t.Parallel()

	// Arrange
	subcommand := "this-is-not-a-go-subcommand"

	// Act
	_, err := proc.Run(t.Context(), goProgram, subcommand)

	// Assert
	// The standard error is the whole point: a caller must be able to show why
	// the command failed without running it again by hand.
	if err == nil || !strings.Contains(err.Error(), subcommand) {
		t.Errorf("Run error = %v, want it to carry the standard error naming %q", err, subcommand)
	}
}

func TestAvailableDistinguishesInstalledFromMissing(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{goProgram: true, missingProgram: false}

	for program, want := range cases {
		t.Run(program, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := proc.Available(program); got != want {
				t.Errorf("Available(%q) = %v, want %v", program, got, want)
			}
		})
	}
}

func TestLookPathFindsAProgram(t *testing.T) {
	t.Parallel()

	// Act
	path, err := proc.LookPath(goProgram)

	// Assert
	if err != nil || path == "" {
		t.Errorf("LookPath(%q) = %q, %v; want a path", goProgram, path, err)
	}
}

func TestLookPathReportsAMissingProgram(t *testing.T) {
	t.Parallel()

	// Act
	_, err := proc.LookPath(missingProgram)

	// Assert
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("LookPath(%q) returned %v, want ErrNotFound", missingProgram, err)
	}
}
