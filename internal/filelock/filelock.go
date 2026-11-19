// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

// Package filelock takes an advisory lock on an open file, which the system
// lets go when the file is closed, whether by the process or by its end, so no
// lock outlives its holder. It is the platform glue alone, in a package of its
// own, so the code that waits on a lock is measured where it lives.
package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrHeld reports a lock another open file holds.
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
