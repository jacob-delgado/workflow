// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestLogReportsAFileItCannotOpen(t *testing.T) {
	// Arrange
	// --log names a file whose parent directory does not exist, so opening the
	// log fails before the interface starts. The root command opens the log
	// before it reaches the terminal, which is what makes this reachable at all.
	logPath := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

	// Act
	_, err := run(t, t.TempDir(), "--log", logPath)

	// Assert
	// The error must name the request log: bare `workflow` also fails when it
	// cannot open a TTY, so a plain "an error occurred" check would pass whether
	// or not the log path was the cause.
	if err == nil || !strings.Contains(err.Error(), "request log") {
		t.Errorf("--log = %v, want it to fail opening the log in a missing directory", err)
	}
}
