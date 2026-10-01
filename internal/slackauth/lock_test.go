// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/slackauth"
)

func TestALockHeldElsewhereIsWaitedFor(t *testing.T) {
	t.Parallel()

	// Arrange
	lock := slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock"))

	unlock, err := lock(t.Context())
	if err != nil {
		t.Fatalf("the first lock = %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// Act
	_, err = lock(ctx)

	// Assert
	if !errors.Is(err, slackauth.ErrLocked) {
		t.Errorf("a second lock = %v, want %v while the first is held", err, slackauth.ErrLocked)
	}

	unlock()
}

func TestALockLeftBehindLongAgoIsTakenOver(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "slack.lock")

	err := os.WriteFile(path, nil, 0o600)
	if err != nil {
		t.Fatalf("writing a stale lock: %v", err)
	}

	old := time.Now().Add(-time.Hour)

	err = os.Chtimes(path, old, old)
	if err != nil {
		t.Fatalf("aging the lock: %v", err)
	}

	// Act
	unlock, err := slackauth.FileLock(path)(t.Context())
	// Assert
	if err != nil {
		t.Fatalf("lock = %v, want a lock an hour old taken over", err)
	}

	unlock()
}
