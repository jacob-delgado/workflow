// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/filelock"
)

// ErrSaveLocked reports a save lock another holder kept for as long as a
// save waits on it.
var ErrSaveLocked = errors.New("another workflow is saving the configuration; try again in a moment")

const (
	// saveLockWait is how long a save waits on a lock before giving up: a
	// save holds one only to read, compare and rename a small file.
	saveLockWait = 10 * time.Second
	// saveLockPoll is how often a waiting save looks again.
	saveLockPoll = 50 * time.Millisecond
	// saveLockDirMode keeps the directory the locks live in to its owner.
	saveLockDirMode = 0o700
)

// SaveLocks are the cross-process locks a save of the configuration files
// holds from its revision check through its rename, so two workflows saving
// over one revision never both write: one for each file a save reads, kept
// in workflow's state directory under a name made from the file's path, so
// nothing is added beside a file in a working tree.
type SaveLocks struct {
	dir string
}

// SaveLocksIn are the locks kept in dir, workflow's state directory. Where
// there is none, dir is "", and a save takes no lock.
func SaveLocksIn(dir string) SaveLocks {
	return SaveLocks{dir: dir}
}

// Hold takes the lock of each file files names, the home file's first, so two
// holders never wait on each other, and the func it returns lets them go.
// Another holder, in this workflow or another, is waited on for no longer
// than saveLockWait, then refused with ErrSaveLocked.
func (l SaveLocks) Hold(files Files) (func(), error) {
	// Trade-off TRADE-64: with no state directory there is nowhere outside
	// the working tree to keep a lock, so the save takes none, and two
	// workflows saving at once can each pass the revision check.
	if l.dir == "" {
		return func() {}, nil
	}

	paths, err := l.lockPaths(files)
	if err != nil {
		return nil, err
	}

	var held []*os.File

	release := func() {
		for _, file := range held {
			_ = file.Close()
		}
	}

	for _, path := range paths {
		file, err := takeSaveLock(path)
		if err != nil {
			release()

			return nil, err
		}

		held = append(held, file)
	}

	return release, nil
}

// lockPaths are the lock files for what files names, in the order they are
// taken: one for each file the links lead to, so a file reached two ways is
// locked once.
func (l SaveLocks) lockPaths(files Files) ([]string, error) {
	var paths []string

	for _, path := range files.Each() {
		target, err := followLinks(path)
		if err != nil {
			return nil, fmt.Errorf("locking %s: %w", path, err)
		}

		sum := sha256.Sum256([]byte(target))
		lock := filepath.Join(l.dir, "config-"+hex.EncodeToString(sum[:16])+".lock")

		if !slices.Contains(paths, lock) {
			paths = append(paths, lock)
		}
	}

	return paths, nil
}

// takeSaveLock opens the lock file at path and takes its lock, looking again
// every saveLockPoll while another holds it, for no longer than saveLockWait.
func takeSaveLock(path string) (*os.File, error) {
	err := os.MkdirAll(filepath.Dir(path), saveLockDirMode)
	if err != nil {
		return nil, fmt.Errorf("making the save lock's directory: %w", err)
	}

	//nolint:gosec // the path is a lock in workflow's own state directory
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FileMode)
	if err != nil {
		return nil, fmt.Errorf("opening the save lock: %w", err)
	}

	err = waitForSaveLock(file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}

	return file, nil
}

// waitForSaveLock takes the lock on file, looking again every saveLockPoll
// while another holds it, for no longer than saveLockWait.
func waitForSaveLock(file *os.File) error {
	deadline := time.Now().Add(saveLockWait)

	for {
		err := filelock.TryLock(file)
		if !errors.Is(err, filelock.ErrHeld) {
			return err
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w (%s)", ErrSaveLocked, file.Name())
		}

		time.Sleep(saveLockPoll)
	}
}
