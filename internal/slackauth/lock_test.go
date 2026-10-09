// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestALockFileNoOneHoldsIsTakenAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// A process that ended mid-refresh leaves its lock file behind, as new as
	// the refresh it was making.
	path := filepath.Join(t.TempDir(), "slack.lock")

	err := os.WriteFile(path, nil, 0o600)
	if err != nil {
		t.Fatalf("writing a lock file left behind: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// Act
	unlock, err := slackauth.FileLock(path)(ctx)
	// Assert
	if err != nil {
		t.Fatalf("lock = %v, want a lock file no one holds taken at once", err)
	}

	unlock()
}

func TestAHolderPastAMinuteKeepsItsLock(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), "slack.lock")

	unlock, err := slackauth.FileLock(path)(t.Context())
	if err != nil {
		t.Fatalf("the first lock = %v", err)
	}
	defer unlock()

	old := time.Now().Add(-time.Hour)

	err = os.Chtimes(path, old, old)
	if err != nil {
		t.Fatalf("aging the lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// Act
	_, err = slackauth.FileLock(path)(ctx)

	// Assert
	if !errors.Is(err, slackauth.ErrLocked) {
		t.Errorf("a second lock = %v, want %v while a holder an hour in holds it", err, slackauth.ErrLocked)
	}
}

func TestAnUnlockedLockIsTakenByTheNext(t *testing.T) {
	t.Parallel()

	// Arrange
	lock := slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock"))

	unlock, err := lock(t.Context())
	if err != nil {
		t.Fatalf("the first lock = %v", err)
	}

	unlock()

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// Act
	next, err := lock(ctx)
	// Assert
	if err != nil {
		t.Fatalf("the next lock = %v, want the lock let go taken", err)
	}

	next()
}

func TestALockThatCannotBeOpenedIsReported(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		lockAt func(t *testing.T) string
		want   string
	}{
		"its directory cannot be made": {
			lockAt: func(t *testing.T) string {
				t.Helper()

				// A regular file where the lock's directory should be: no
				// directory can be made inside a file, whoever asks.
				parent := filepath.Join(t.TempDir(), "not-a-directory")

				err := os.WriteFile(parent, []byte("a file"), 0o600)
				if err != nil {
					t.Fatalf("writing the file in the way: %v", err)
				}

				return filepath.Join(parent, "state", "slack.lock")
			},
			want: "making the lock's directory",
		},
		"a directory is where it goes": {
			lockAt: func(t *testing.T) string {
				t.Helper()

				return t.TempDir()
			},
			want: "opening the lock",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			lock := slackauth.FileLock(tt.lockAt(t))

			// Act
			unlock, err := lock(t.Context())

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) || errors.Is(err, slackauth.ErrLocked) {
				t.Errorf("lock = %v, want %q reported rather than a lock held elsewhere", err, tt.want)
			}

			if unlock != nil {
				t.Error("lock gave an unlock, want none for a lock it never took")
			}
		})
	}
}
