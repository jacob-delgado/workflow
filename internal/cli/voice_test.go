// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

func TestAFailedCommandSaysItsErrorOnceTagged(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "launch")

	// Assert
	wantExit(t, err, 2)

	if strings.Count(printed.stderr, "ERROR") != 1 ||
		!strings.HasPrefix(printed.stderr, `ERROR unknown command "launch"`) {
		t.Errorf("stderr = %q, want the error said once, after its tag", printed.stderr)
	}

	if strings.Contains(printed.stderr, "workflow:") || printed.stdout != "" {
		t.Errorf("stdout %q, stderr %q; want no workflow: prefix, and nothing on stdout", printed.stdout, printed.stderr)
	}
}

func TestASucceededCommandSaysNoError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--version")

	// Assert
	if err != nil || strings.Contains(printed.stderr, "ERROR") {
		t.Errorf("--version = %v, stderr %q; want no error said", err, printed.stderr)
	}
}
