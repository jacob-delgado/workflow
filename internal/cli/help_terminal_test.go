// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The help is colored on a terminal, which only a pseudo-terminal can stand
// in for. Fang reads the terminal's variables from the process itself, so
// this file's test sets TERM there, and runs alone, as .golangci.yml's
// paralleltest exclusion for it says.

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/ptytest"
)

func TestHelpIsColoredOnATerminal(t *testing.T) {
	// Arrange
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")

	typing, terminal := ptytest.Open(t)

	var shown bytes.Buffer

	read := make(chan struct{})

	go func() {
		defer close(read)

		_, _ = io.Copy(&shown, typing)
	}()

	// Act
	err := cli.Execute([]string{doctorCommand, "--help"}, terminal, io.Discard, unusedPrompt(t),
		environmentFor(t, place{dir: t.TempDir(), home: t.TempDir()}))

	// Assert
	_ = terminal.Close()

	<-read

	if err != nil || !strings.Contains(shown.String(), "\x1b[") || !strings.Contains(shown.String(), "USAGE") {
		t.Errorf("doctor --help on a terminal = %v, showing %q; want it colored", err, shown.String())
	}
}
