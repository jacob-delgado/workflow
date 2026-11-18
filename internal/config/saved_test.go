// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

// A file of the user's own that a first run replaces is saved under its lock
// with what it held before, so the write can be taken back when the keychain
// then refuses the token it reads.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
)

// replacedContents is the file a ReplaceFile test replaces.
const replacedContents = `{"jira": {"base_url": "https://old.example.com", "token": "old"}}` + "\n"

func TestReplaceFileWritesNothingWhileAnotherWorkflowHoldsTheLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		path := write(t, t.TempDir(), replacedContents)
		locks, _ := heldElsewhere(t, config.Files{Home: path})

		// Act
		_, err := config.ReplaceFile(locks, path, savedConfig())

		// Assert
		contents, readErr := os.ReadFile(path)
		if !errors.Is(err, config.ErrSaveLocked) || readErr != nil || string(contents) != replacedContents {
			t.Errorf("ReplaceFile = %v, the file %q (%v); want %v and the file as it was",
				err, contents, readErr, config.ErrSaveLocked)
		}
	})
}

func TestReplaceFileInADirectoryNothingCanBeWrittenInIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root writes in any directory")
	}

	dir := t.TempDir()
	path := write(t, dir, replacedContents)

	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("making the directory read-only: %v", err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	// Act
	_, err = config.ReplaceFile(config.SaveLocksIn(""), path, savedConfig())

	// Assert
	contents, readErr := os.ReadFile(path)
	if !errors.Is(err, fs.ErrPermission) || readErr != nil || string(contents) != replacedContents {
		t.Errorf("ReplaceFile = %v, the file %q (%v); want the permission refused and the file as it was",
			err, contents, readErr)
	}
}

func TestReplaceFileBeneathAFileIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// What should be a directory on the way to the file is a file.
	path := filepath.Join(write(t, t.TempDir(), replacedContents), config.FileName)

	// Act
	_, err := config.ReplaceFile(config.SaveLocksIn(""), path, savedConfig())

	// Assert
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("ReplaceFile = %v, want the path refused as no directory", err)
	}
}

func TestAReplacedFileTakenBackHoldsWhatItHeldThroughItsLink(t *testing.T) {
	t.Parallel()

	// Arrange
	// The file is a dotfiles checkout's link.
	target := write(t, t.TempDir(), replacedContents)
	path := filepath.Join(t.TempDir(), config.FileName)
	locks := config.SaveLocksIn(t.TempDir())

	err := os.Symlink(target, path)
	if err != nil {
		t.Fatalf("linking the file: %v", err)
	}

	saved, err := config.ReplaceFile(locks, path, savedConfig())
	if err != nil {
		t.Fatalf("ReplaceFile = %v", err)
	}

	// Act
	err = saved.TakeBack(locks)

	// Assert
	info, statErr := os.Lstat(path)
	contents, readErr := os.ReadFile(target)

	if err != nil || statErr != nil || info.Mode()&fs.ModeSymlink == 0 || readErr != nil ||
		string(contents) != replacedContents {
		t.Errorf("TakeBack = %v, the link %v, its target %q (%v); want the link kept and its target as it was",
			err, statErr, contents, readErr)
	}
}

func TestAFileReplaceFileMadeTakenBackGoes(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)
	locks := config.SaveLocksIn(t.TempDir())

	saved, err := config.ReplaceFile(locks, path, savedConfig())
	if err != nil {
		t.Fatalf("ReplaceFile = %v", err)
	}

	// Act
	err = saved.TakeBack(locks)

	// Assert
	_, statErr := os.Lstat(path)
	if err != nil || !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("TakeBack = %v, the file %v; want the file the write made gone", err, statErr)
	}
}
