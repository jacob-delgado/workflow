// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cli_test

// SecretReader reads from a terminal, which only a pseudo-terminal can stand
// in for; Linux makes one through /dev/ptmx, so these tests are Linux's.

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// openTerminal is a pseudo-terminal: the side that types into it, and the
// terminal a program reads from. Both close when the test ends.
func openTerminal(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	typing, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal to read from here: %v", err)
	}

	t.Cleanup(func() { _ = typing.Close() })

	descriptor := int(typing.Fd())

	err = unix.IoctlSetPointerInt(descriptor, unix.TIOCSPTLCK, 0)
	if err != nil {
		t.Fatalf("unlocking the pseudo-terminal: %v", err)
	}

	number, err := unix.IoctlGetInt(descriptor, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("naming the pseudo-terminal: %v", err)
	}

	terminal, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("opening the pseudo-terminal: %v", err)
	}

	t.Cleanup(func() { _ = terminal.Close() })

	return typing, terminal
}

func TestSecretReaderReadsTheLineTypedAtATerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	typing, terminal := openTerminal(t)

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
	_, terminal := openTerminal(t)

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
