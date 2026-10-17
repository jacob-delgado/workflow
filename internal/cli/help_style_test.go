// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestHelpIsLaidOutInSectionsAndPlainOnAPipe(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), doctorCommand, "--help")

	// Assert
	if err != nil || printed.Stderr != "" {
		t.Fatalf("doctor --help = %v, stderr %q; want the help alone", err, printed.Stderr)
	}

	for _, section := range []string{"USAGE", "EXAMPLES", "FLAGS"} {
		if !strings.Contains(printed.Stdout, section) {
			t.Errorf("doctor --help has no %s section:\n%s", section, printed.Stdout)
		}
	}

	if strings.Contains(printed.Stdout, "\x1b[") {
		t.Errorf("doctor --help on a pipe carries escapes: %q", printed.Stdout)
	}
}

func TestTheVersionFlagPrintsTheBuildAlone(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--version")

	// Assert
	if err != nil || printed.Stdout != "workflow version "+buildinfo.Current()+"\n" || printed.Stderr != "" {
		t.Errorf("--version = %v, stdout %q, stderr %q; want the one line cobra has always printed",
			err, printed.Stdout, printed.Stderr)
	}
}
