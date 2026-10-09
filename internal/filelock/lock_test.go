// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package filelock_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/filelock"
)

// opened is the lock file at path opened once more, closed when the test ends.
func opened(t *testing.T, path string) *os.File {
	t.Helper()

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("opening the lock file: %v", err)
	}

	t.Cleanup(func() { _ = file.Close() })

	return file
}

func TestTryLockTakesAFileNoOneHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	file := opened(t, filepath.Join(t.TempDir(), "lock"))

	// Act
	err := filelock.TryLock(file)
	// Assert
	if err != nil {
		t.Errorf("TryLock = %v; want the lock taken", err)
	}
}

func TestTryLockLeavesALockAnotherOpenFileHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "lock")

	err := filelock.TryLock(opened(t, path))
	if err != nil {
		t.Fatalf("the first TryLock = %v", err)
	}

	// Act
	err = filelock.TryLock(opened(t, path))

	// Assert
	if !errors.Is(err, filelock.ErrHeld) {
		t.Errorf("a second TryLock = %v; want %v, the lock left to its holder", err, filelock.ErrHeld)
	}
}

func TestClosingTheFileLetsItsLockGo(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "lock")
	first := opened(t, path)

	err := filelock.TryLock(first)
	if err != nil {
		t.Fatalf("the first TryLock = %v", err)
	}

	err = first.Close()
	if err != nil {
		t.Fatalf("closing the first: %v", err)
	}

	// Act
	err = filelock.TryLock(opened(t, path))
	// Assert
	if err != nil {
		t.Errorf("TryLock after the holder closed = %v; want the lock taken", err)
	}
}

func TestTryLockReportsAFileItCannotLock(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "lock")
	file := opened(t, path)

	err := file.Close()
	if err != nil {
		t.Fatalf("closing the file: %v", err)
	}

	// Act
	err = filelock.TryLock(file)

	// Assert
	if err == nil || errors.Is(err, filelock.ErrHeld) || !strings.Contains(err.Error(), path) {
		t.Errorf("TryLock on a closed file = %v; want a failure other than %v, naming %s", err, filelock.ErrHeld, path)
	}
}
