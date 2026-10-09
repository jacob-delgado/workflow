// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package ptytest

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// terminalNameSize is the room TIOCPTYGNAME writes a terminal's name into:
// the parameter length its request code encodes.
const terminalNameSize = 128

// openPair opens /dev/ptmx, grants and unlocks the terminal it made, and
// opens that terminal by the name macOS gives it.
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

// openTerminal grants, unlocks and opens the terminal of the multiplexer at
// descriptor.
func openTerminal(descriptor int) (*os.File, error) {
	err := unix.IoctlSetInt(descriptor, unix.TIOCPTYGRANT, 0)
	if err != nil {
		return nil, fmt.Errorf("granting the terminal: %w", err)
	}

	err = unix.IoctlSetInt(descriptor, unix.TIOCPTYUNLK, 0)
	if err != nil {
		return nil, fmt.Errorf("unlocking the terminal: %w", err)
	}

	name := make([]byte, terminalNameSize)

	//nolint:gosec // TIOCPTYGNAME writes the name into the buffer it is handed
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(descriptor), uintptr(unix.TIOCPTYGNAME),
		uintptr(unsafe.Pointer(&name[0])))
	if errno != 0 {
		return nil, fmt.Errorf("naming the terminal: %w", errno)
	}

	name, _, _ = bytes.Cut(name, []byte{0})

	terminal, err := os.OpenFile(string(name), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("opening the terminal: %w", err)
	}

	return terminal, nil
}
