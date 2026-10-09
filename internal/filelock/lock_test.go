// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package filelock_test

import (
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
	held, err := filelock.TryLock(file)

	// Assert
	if err != nil || !held {
		t.Errorf("TryLock = %t, %v; want the lock taken", held, err)
	}
}

func TestTryLockLeavesALockAnotherOpenFileHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "lock")

	held, err := filelock.TryLock(opened(t, path))
	if err != nil || !held {
		t.Fatalf("the first TryLock = %t, %v", held, err)
	}

	// Act
	again, err := filelock.TryLock(opened(t, path))

	// Assert
	if err != nil || again {
		t.Errorf("a second TryLock = %t, %v; want the lock left to its holder", again, err)
	}
}

func TestClosingTheFileLetsItsLockGo(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "lock")
	first := opened(t, path)

	held, err := filelock.TryLock(first)
	if err != nil || !held {
		t.Fatalf("the first TryLock = %t, %v", held, err)
	}

	err = first.Close()
	if err != nil {
		t.Fatalf("closing the first: %v", err)
	}

	// Act
	next, err := filelock.TryLock(opened(t, path))

	// Assert
	if err != nil || !next {
		t.Errorf("TryLock after the holder closed = %t, %v; want the lock taken", next, err)
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
	held, err := filelock.TryLock(file)

	// Assert
	if err == nil || held || !strings.Contains(err.Error(), path) {
		t.Errorf("TryLock on a closed file = %t, %v; want a failure naming %s", held, err, path)
	}
}
