// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// openRequestLog is reached only from the root command, which runs the terminal
// interface and so cannot run in a test. These tests live in package cli —
// testpackage's default skip covers *_internal_test.go — to reach it directly.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRequestLogWithNoPathIsANoOp(t *testing.T) {
	t.Parallel()

	// Act
	log, closeLog, err := openRequestLog("")

	// Assert
	if log != nil || err != nil {
		t.Errorf("openRequestLog(\"\") = %v, %v, want no log and no error", log, err)
	}

	closeLog() // the no-op close must be safe to call
}

func TestOpenRequestLogCreatesTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "requests.log")

	// Act
	log, closeLog, err := openRequestLog(path)
	if err != nil {
		t.Fatalf("openRequestLog returned %v, want nil", err)
	}

	defer closeLog()

	// Assert
	if log == nil {
		t.Fatal("openRequestLog returned no log for a path")
	}

	_, err = os.Stat(path)
	if err != nil {
		t.Errorf("the log file was not created: %v", err)
	}
}

func TestOpenRequestLogReportsAFileItCannotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path whose parent directory does not exist: the open is what fails.
	path := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

	// Act
	_, _, err := openRequestLog(path)

	// Assert
	if err == nil {
		t.Error("openRequestLog opened a file in a missing directory, want an error")
	}
}
