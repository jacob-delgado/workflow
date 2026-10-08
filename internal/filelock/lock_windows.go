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

// TryLock takes an exclusive lock on file's first byte without waiting, and
// reports whether it holds it: false while another open file holds one.
func TryLock(file *os.File) (bool, error) {
	var whole windows.Overlapped

	err := windows.LockFileEx(windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &whole)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("locking %s: %w", file.Name(), err)
	}

	return true, nil
}
