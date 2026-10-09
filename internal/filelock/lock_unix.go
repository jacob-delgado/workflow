// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrHeld reports a lock another open file holds. It is declared beside
// each platform's TryLock, since a file of build-tagged twins must stand alone
// for the condition coverage report to read it.
var ErrHeld = errors.New("another open file holds the lock")

// TryLock takes an exclusive lock on file without waiting, and fails with
// ErrHeld while another open file holds one.
func TryLock(file *os.File) error {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return fmt.Errorf("%w: %s", ErrHeld, file.Name())
	}

	if err != nil {
		return fmt.Errorf("locking %s: %w", file.Name(), err)
	}

	return nil
}
