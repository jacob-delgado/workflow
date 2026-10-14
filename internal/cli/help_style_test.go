// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
)

func TestHelpIsLaidOutInSectionsAndPlainOnAPipe(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), doctorCommand, "--help")

	// Assert
	if err != nil || printed.stderr != "" {
		t.Fatalf("doctor --help = %v, stderr %q; want the help alone", err, printed.stderr)
	}

	for _, section := range []string{"USAGE", "EXAMPLES", "FLAGS"} {
		if !strings.Contains(printed.stdout, section) {
			t.Errorf("doctor --help has no %s section:\n%s", section, printed.stdout)
		}
	}

	if strings.Contains(printed.stdout, "\x1b[") {
		t.Errorf("doctor --help on a pipe carries escapes: %q", printed.stdout)
	}
}

func TestTheVersionFlagPrintsTheBuildAlone(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--version")

	// Assert
	if err != nil || printed.stdout != "workflow version "+buildinfo.Current()+"\n" || printed.stderr != "" {
		t.Errorf("--version = %v, stdout %q, stderr %q; want the one line cobra has always printed",
			err, printed.stdout, printed.stderr)
	}
}
