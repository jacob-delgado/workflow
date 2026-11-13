// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// heldElsewhere is the save locks kept in a state directory of the test's
// own, with files' locks held as another workflow would hold them until the
// test ends or it lets them go.
func heldElsewhere(t *testing.T, files config.Files) (config.SaveLocks, func()) {
	t.Helper()

	locks := config.SaveLocksIn(t.TempDir())

	unlock, err := locks.Hold(files)
	if err != nil {
		t.Fatalf("holding the save locks: %v", err)
	}

	t.Cleanup(unlock)

	return locks, unlock
}

// projectLayer is a repository file setting the Jira project alone.
const projectLayer = `{"jira": {"project": "OSS"}}`

func TestASaveWritesNothingWhileAnotherWorkflowHoldsTheLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, cfg, over := readLayered(t, projectLayer)
		locks, _ := heldElsewhere(t, files)
		cfg.Messaging.Channel = devChannel

		// Act
		_, err := config.SaveLayers(locks, files, cfg, over)

		// Assert
		now, revErr := config.RevisionOfLayers(files)
		if !errors.Is(err, config.ErrSaveLocked) || revErr != nil || now != over {
			t.Errorf("SaveLayers = %v; the files are as they were: %t (%v); want %v and nothing written while "+
				"another workflow held the lock", err, now == over, revErr, config.ErrSaveLocked)
		}
	})
}

func TestASaveWaitsForTheLockAndThenWrites(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, cfg, over := readLayered(t, projectLayer)
		locks, unlock := heldElsewhere(t, files)
		cfg.Messaging.Channel = devChannel
		started := time.Now()
		released := make(chan struct{})

		go func() {
			defer close(released)

			time.Sleep(time.Second)
			unlock()
		}()

		// Act
		_, err := config.SaveLayers(locks, files, cfg, over)

		// Assert
		waited := time.Since(started)

		<-released

		if err != nil || waited < time.Second || reread(t, files).Messaging.Channel != devChannel {
			t.Errorf("SaveLayers = %v after %v; want the save written once the holder let go after 1s", err, waited)
		}
	})
}

func TestCreatingALayerWritesNothingWhileAnotherWorkflowHoldsTheLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files := config.Files{Home: write(t, t.TempDir(), homeFile), Repo: filepath.Join(t.TempDir(), config.FileName)}

		cfg, over, err := config.LoadLayersAt(files)
		if err != nil {
			t.Fatalf("LoadLayersAt: %v", err)
		}

		locks, _ := heldElsewhere(t, files)

		// Act
		err = config.CreateLayers(locks, files, cfg, over)

		// Assert
		_, statErr := os.Lstat(files.Repo)
		if !errors.Is(err, config.ErrSaveLocked) || !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("CreateLayers = %v, the repository file: %v; want %v and no file made", err, statErr,
				config.ErrSaveLocked)
		}
	})
}

func TestAWriteTakenBackWaitsForTheLockAndLeavesTheFileWhenItCannot(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, read, revision := readHome(t)
		edited := read.Redacted()
		edited.Messaging.RefreshToken, edited.Jira.Project = typedRefresh, ossProject
		locks := config.SaveLocksIn(t.TempDir())

		refusedWhileHeld := func(config.Config) error {
			unlock, err := locks.Hold(files)
			if err != nil {
				t.Fatalf("holding the save locks: %v", err)
			}

			t.Cleanup(unlock)

			return errSlackRefused
		}

		// Act
		_, _, err := config.SaveEdit(config.Edit{
			Files: files, Locks: locks, Read: read, Over: revision, Edited: edited,
			PlaceSlackCredentials: refusedWhileHeld,
		})

		// Assert
		if !errors.Is(err, errSlackRefused) || !errors.Is(err, config.ErrSaveLocked) ||
			reread(t, files).Jira.Project != ossProject {
			t.Errorf("SaveEdit = %v; want Slack's refusal and %v, the write left in place while another held the lock",
				err, config.ErrSaveLocked)
		}
	})
}

func TestASaveAddsNothingBesideTheFilesAndKeepsItsLockInTheStateDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	files, cfg, over := readLayered(t, projectLayer)
	stateDir := t.TempDir()
	cfg.Messaging.Channel = devChannel

	// Act
	_, err := config.SaveLayers(config.SaveLocksIn(stateDir), files, cfg, over)

	// Assert
	beside, readErr := os.ReadDir(filepath.Dir(files.Repo))
	kept, keptErr := os.ReadDir(stateDir)

	if err != nil || readErr != nil || keptErr != nil || len(beside) != 2 || len(kept) == 0 {
		t.Errorf("SaveLayers = %v; the repository holds %v (%v), the state directory %v (%v); want only the file "+
			"and the repository marker beside it, and the lock in the state directory", err, beside, readErr, kept,
			keptErr)
	}
}

func TestWithNoStateDirectoryASaveTakesNoLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, cfg, over := readLayered(t, projectLayer)
		none := config.SaveLocksIn("")

		unlock, err := none.Hold(files)
		if err != nil {
			t.Fatalf("Hold: %v", err)
		}

		defer unlock()

		started := time.Now()

		// Act
		_, err = config.SaveLayers(none, files, cfg, over)

		// Assert
		if err != nil || time.Since(started) != 0 {
			t.Errorf("SaveLayers = %v after %v; want the save written at once, with no lock to wait on", err,
				time.Since(started))
		}
	})
}

func TestAFileReachedTwoWaysIsLockedOnce(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		target := write(t, t.TempDir(), homeFile)
		link := filepath.Join(t.TempDir(), config.FileName)

		err := os.Symlink(target, link)
		if err != nil {
			t.Fatalf("linking the home file: %v", err)
		}

		started := time.Now()

		// Act
		unlock, err := config.SaveLocksIn(t.TempDir()).Hold(config.Files{Home: link, Repo: target})

		// Assert
		if err != nil || time.Since(started) != 0 {
			t.Errorf("Hold = %v after %v; want the one file locked once, at once", err, time.Since(started))
		}

		if unlock != nil {
			unlock()
		}
	})
}

func TestHoldReportsAFileWhosePathCannotBeFollowed(t *testing.T) {
	t.Parallel()

	// Arrange
	notADirectory := write(t, t.TempDir(), homeFile)
	files := config.Files{Home: filepath.Join(notADirectory, config.FileName)}

	// Act
	_, err := config.SaveLocksIn(t.TempDir()).Hold(files)

	// Assert
	if err == nil || errors.Is(err, config.ErrSaveLocked) {
		t.Errorf("Hold = %v; want the path that cannot be followed reported", err)
	}
}

func TestHoldReportsAStateDirectoryItCannotMake(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: write(t, t.TempDir(), homeFile)}
	underAFile := filepath.Join(write(t, t.TempDir(), homeFile), "state")

	// Act
	_, err := config.SaveLocksIn(underAFile).Hold(files)

	// Assert
	if err == nil || errors.Is(err, config.ErrSaveLocked) {
		t.Errorf("Hold = %v; want the state directory that cannot be made reported", err)
	}
}

func TestHoldReportsALockItCannotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root opens a file in a directory whatever its mode, so the lock would open")
	}

	files := config.Files{Home: write(t, t.TempDir(), homeFile)}
	stateDir := t.TempDir()
	sealDirectory(t, stateDir)

	// Act
	_, err := config.SaveLocksIn(stateDir).Hold(files)

	// Assert
	if err == nil || errors.Is(err, config.ErrSaveLocked) {
		t.Errorf("Hold = %v; want the lock that cannot be opened reported", err)
	}
}
