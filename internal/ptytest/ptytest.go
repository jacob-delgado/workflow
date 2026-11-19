// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package ptytest opens a pseudo-terminal for a test: the side the test types
// into, and the terminal the code under test reads from, which only a
// pseudo-terminal can stand in for. Linux and macOS each make one their own
// way, so each way lives here, in a file of its own. Only tests import this
// package.
package ptytest

import (
	"errors"
	"os"
	"testing"
)

// errNoPseudoTerminal is a system on which no pseudo-terminal can be opened,
// as one the test runs on without any.
var errNoPseudoTerminal = errors.New("no pseudo-terminal to open here")

// Open opens a pseudo-terminal: the side that types into it, and the terminal
// a program reads from. Both close when the test ends. Where none can be
// opened, the test is skipped; one opened that cannot be named or unlocked
// fails it.
func Open(tb testing.TB) (*os.File, *os.File) {
	tb.Helper()

	typing, terminal, err := openPair()
	if errors.Is(err, errNoPseudoTerminal) {
		tb.Skipf("typing at a terminal: %v", err)
	}

	if err != nil {
		tb.Fatalf("opening a pseudo-terminal: %v", err)
	}

	tb.Cleanup(func() {
		_ = terminal.Close()
		_ = typing.Close()
	})

	return typing, terminal
}
