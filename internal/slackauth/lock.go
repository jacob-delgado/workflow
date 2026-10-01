// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked reports a refresh lock another process held for as long as this one
// would wait.
var ErrLocked = errors.New("another workflow is refreshing the Slack token")

const (
	// lockWait is how long a refresh waits on a lock before giving up.
	lockWait = 10 * time.Second
	// lockStale is how old a lock is before it is taken as left behind by a
	// process that ended mid-refresh: far longer than any refresh takes.
	lockStale = time.Minute
	// lockPoll is how often a waiting refresh looks again.
	lockPoll = 50 * time.Millisecond
	// lockDirMode and lockFileMode keep the lock to its owner.
	lockDirMode  = 0o700
	lockFileMode = 0o600
)

// FileLock is a lock that is a file at path, made only where none is: one
// process at a time holds it, across every workflow running. A lock older than
// lockStale is taken over, and a wait gives up after lockWait or when ctx ends.
func FileLock(path string) func(ctx context.Context) (func(), error) {
	return func(ctx context.Context) (func(), error) {
		err := os.MkdirAll(filepath.Dir(path), lockDirMode)
		if err != nil {
			return nil, fmt.Errorf("making the lock's directory: %w", err)
		}

		ctx, cancel := context.WithTimeout(ctx, lockWait)
		defer cancel()

		for {
			unlock, taken := tryLock(path)
			if taken {
				return unlock, nil
			}

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("%w (%s)", ErrLocked, path)
			case <-time.After(lockPoll):
			}
		}
	}
}

// tryLock makes the lock file, or takes over one left behind, and reports
// whether it holds the lock.
func tryLock(path string) (func(), bool) {
	//nolint:gosec // the path is the lock wiring names in the user's own data directory
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, lockFileMode)
	if err == nil {
		_ = file.Close()

		return func() { _ = os.Remove(path) }, true
	}

	info, statErr := os.Stat(path)
	if statErr == nil && time.Since(info.ModTime()) > lockStale {
		_ = os.Remove(path)
	}

	return nil, false
}
