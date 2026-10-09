// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package filelock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// ErrHeld reports a lock another open file holds. It is declared beside
// each platform's TryLock, since a file of build-tagged twins must stand alone
// for the condition coverage report to read it.
var ErrHeld = errors.New("another open file holds the lock")

// TryLock takes an exclusive lock on file's first byte without waiting, and
// fails with ErrHeld while another open file holds one.
func TryLock(file *os.File) error {
	var whole windows.Overlapped

	err := windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &whole)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return fmt.Errorf("%w: %s", ErrHeld, file.Name())
	}

	if err != nil {
		return fmt.Errorf("locking %s: %w", file.Name(), err)
	}

	return nil
}
