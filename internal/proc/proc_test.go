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

	output, err := proc.Run(t.Context(), goProgram, "version")
	if err != nil {
		t.Fatalf("Run(go version) returned %v, want nil", err)
	}

	if !strings.Contains(string(output), "go version") {
		t.Errorf("Run(go version) = %q, want it to contain %q", output, "go version")
	}
}

func TestRunReportsAProgramNotOnPath(t *testing.T) {
	t.Parallel()

	_, err := proc.Run(t.Context(), missingProgram)
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Run(%q) returned %v, want ErrNotFound", missingProgram, err)
	}
}

func TestRunReportsAFailureWithItsStandardError(t *testing.T) {
	t.Parallel()

	subcommand := "this-is-not-a-go-subcommand"

	_, err := proc.Run(t.Context(), goProgram, subcommand)
	if err == nil {
		t.Fatal("Run(go this-is-not-a-go-subcommand) returned nil, want an error")
	}

	// The standard error is the whole point: a caller must be able to show why
	// the command failed without running it again by hand.
	if !strings.Contains(err.Error(), subcommand) {
		t.Errorf("Run error = %q, want it to carry the standard error naming %q", err, subcommand)
	}
}

func TestAvailableDistinguishesInstalledFromMissing(t *testing.T) {
	t.Parallel()

	if !proc.Available(goProgram) {
		t.Errorf("Available(%q) = false, want true", goProgram)
	}

	if proc.Available(missingProgram) {
		t.Errorf("Available(%q) = true, want false", missingProgram)
	}
}

func TestLookPathFindsAProgramAndReportsAMissingOne(t *testing.T) {
	t.Parallel()

	path, err := proc.LookPath(goProgram)
	if err != nil {
		t.Fatalf("LookPath(%q) returned %v, want nil", goProgram, err)
	}

	if path == "" {
		t.Errorf("LookPath(%q) returned an empty path", goProgram)
	}

	_, err = proc.LookPath(missingProgram)
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("LookPath(%q) returned %v, want ErrNotFound", missingProgram, err)
	}
}
