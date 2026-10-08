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

// TryLock takes an exclusive lock on file without waiting, and reports
// whether it holds it: false while another open file holds one.
func TryLock(file *os.File) (bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("locking %s: %w", file.Name(), err)
	}

	return true, nil
}
