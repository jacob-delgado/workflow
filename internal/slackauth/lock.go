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

	"github.com/jacob-delgado/workflow/internal/filelock"
)

// ErrLocked reports a refresh lock another process held for as long as this one
// would wait.
var ErrLocked = errors.New("another workflow is refreshing the Slack token; try again in a minute")

const (
	// lockWait is how long a refresh waits on a lock before giving up.
	lockWait = 10 * time.Second
	// lockPoll is how often a waiting refresh looks again.
	lockPoll = 50 * time.Millisecond
	// lockDirMode and lockFileMode keep the lock to its owner.
	lockDirMode  = 0o700
	lockFileMode = 0o600
)

// FileLock is a lock held on the file at path: one process at a time holds
// it, across every workflow running, for as long as it lives or until it
// unlocks. The system lets go of a lock whose holder ended, so a lock file
// left behind is taken at once, and one held for however long is never taken
// over. A wait gives up after lockWait or when ctx ends.
func FileLock(path string) func(ctx context.Context) (func(), error) {
	return func(ctx context.Context) (func(), error) {
		err := os.MkdirAll(filepath.Dir(path), lockDirMode)
		if err != nil {
			return nil, fmt.Errorf("making the lock's directory: %w", err)
		}

		//nolint:gosec // the path is the lock wiring names in the user's own data directory
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockFileMode)
		if err != nil {
			return nil, fmt.Errorf("opening the lock: %w", err)
		}

		err = waitFor(ctx, file)
		if err != nil {
			return nil, errors.Join(err, file.Close())
		}

		return func() { _ = file.Close() }, nil
	}
}

// waitFor takes the lock on file, looking again every lockPoll while another
// holds it, for no longer than lockWait or ctx allows.
func waitFor(ctx context.Context, file *os.File) error {
	ctx, cancel := context.WithTimeout(ctx, lockWait)
	defer cancel()

	for {
		err := filelock.TryLock(file)
		if !errors.Is(err, filelock.ErrHeld) {
			return err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (%s)", ErrLocked, file.Name())
		case <-time.After(lockPoll):
		}
	}
}
