// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package ptytest

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// openPair opens /dev/ptmx, unlocks the terminal it made, and opens that
// terminal by the number Linux gives it under /dev/pts.
func openPair() (*os.File, *os.File, error) {
	typing, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", errNoPseudoTerminal, err)
	}

	terminal, err := openTerminal(int(typing.Fd()))
	if err != nil {
		_ = typing.Close()

		return nil, nil, err
	}

	return typing, terminal, nil
}

// openTerminal unlocks and opens the terminal of the multiplexer at
// descriptor.
func openTerminal(descriptor int) (*os.File, error) {
	err := unix.IoctlSetPointerInt(descriptor, unix.TIOCSPTLCK, 0)
	if err != nil {
		return nil, fmt.Errorf("unlocking the terminal: %w", err)
	}

	number, err := unix.IoctlGetInt(descriptor, unix.TIOCGPTN)
	if err != nil {
		return nil, fmt.Errorf("naming the terminal: %w", err)
	}

	terminal, err := os.OpenFile("/dev/pts/"+strconv.Itoa(number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("opening the terminal: %w", err)
	}

	return terminal, nil
}
