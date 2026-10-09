// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// SecretReader reads from a terminal, which only a pseudo-terminal can stand
// in for.

import (
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/ptytest"
)

func TestSecretReaderReadsTheLineTypedAtATerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	typing, terminal := ptytest.Open(t)

	_, err := typing.WriteString("s3cret-answer\n")
	if err != nil {
		t.Fatalf("typing the answer: %v", err)
	}

	var prompts strings.Builder

	// Act
	answer, err := cli.SecretReader(terminal, &prompts)("Token: ")

	// Assert
	if err != nil || answer != "s3cret-answer" {
		t.Errorf("read = %q, %v; want the line typed, without its newline", answer, err)
	}

	if prompts.String() != "Token: \n" {
		t.Errorf("printed %q, want the prompt, then a newline for the one not echoed", prompts.String())
	}
}

func TestSecretReaderSaysWhyATerminalCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The terminal is opened for writing alone, so reading from it fails.
	_, terminal := ptytest.Open(t)

	writeOnly, err := os.OpenFile(terminal.Name(), os.O_WRONLY|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the pseudo-terminal for writing: %v", err)
	}

	t.Cleanup(func() { _ = writeOnly.Close() })

	// Act
	answer, err := cli.SecretReader(writeOnly, new(strings.Builder))("Token: ")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "reading the answer") || answer != "" {
		t.Errorf("read = %q, %v; want no answer and the read's failure", answer, err)
	}
}
