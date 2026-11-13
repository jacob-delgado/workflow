// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// heldFile is what another workflow writes to a file while it holds the save
// locks.
const heldFile = `{"jira": {"project": "HELD"}}`

// writesWhileHeld holds files' locks as another workflow saving would: it
// writes heldFile to path a second after the call, then lets the locks go. The
// func it returns waits for that and reports the write's failure.
func writesWhileHeld(t *testing.T, locks config.SaveLocks, files config.Files, path string) func() error {
	t.Helper()

	unlock, err := locks.Hold(files)
	if err != nil {
		t.Fatalf("holding the save locks: %v", err)
	}

	done := make(chan error, 1)

	go func() {
		defer unlock()

		time.Sleep(time.Second)

		done <- os.WriteFile(path, []byte(heldFile), config.FileMode)
	}()

	return func() error { return <-done }
}

// holdsHeldFile reports whether path holds what the holder wrote.
func holdsHeldFile(t *testing.T, path string) bool {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return bytes.Equal(got, []byte(heldFile))
}

func TestASaveSeesTheWriteOfTheWorkflowItWaitedFor(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, cfg, over := readLayered(t, projectLayer)
		locks := config.SaveLocksIn(t.TempDir())
		holderWrote := writesWhileHeld(t, locks, files, files.Repo)
		cfg.Messaging.Channel = devChannel

		// Act
		_, err := config.SaveLayers(locks, files, cfg, over)

		// Assert
		writeErr := holderWrote()
		if !errors.Is(err, config.ErrChangedOnDisk) || writeErr != nil || !holdsHeldFile(t, files.Repo) {
			t.Errorf("SaveLayers = %v (the holder's write: %v); want %v, and the holder's write kept: a save "+
				"checks the revision only once it holds the lock", err, writeErr, config.ErrChangedOnDisk)
		}
	})
}

func TestCreatingALayerSeesTheWriteOfTheWorkflowItWaitedFor(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files := config.Files{Home: write(t, t.TempDir(), homeFile), Repo: filepath.Join(t.TempDir(), config.FileName)}

		cfg, over, err := config.LoadLayersAt(files)
		if err != nil {
			t.Fatalf("LoadLayersAt: %v", err)
		}

		locks := config.SaveLocksIn(t.TempDir())
		holderWrote := writesWhileHeld(t, locks, files, files.Home)

		// Act
		err = config.CreateLayers(locks, files, cfg, over)

		// Assert
		writeErr := holderWrote()
		_, statErr := os.Lstat(files.Repo)

		if !errors.Is(err, config.ErrChangedOnDisk) || writeErr != nil || !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("CreateLayers = %v (the holder's write: %v), the repository file: %v; want %v and no file "+
				"made over a home file changed meanwhile: creating checks the revision only once it holds the lock",
				err, writeErr, statErr, config.ErrChangedOnDisk)
		}
	})
}

func TestAWriteTakenBackSeesTheWriteOfTheWorkflowItWaitedFor(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files, read, revision := readHome(t)
		edited := read.Redacted()
		edited.Messaging.RefreshToken, edited.Jira.Project = typedRefresh, ossProject
		locks := config.SaveLocksIn(t.TempDir())

		var holderWrote func() error

		refusedWhileAnotherSaves := func(config.Config) error {
			holderWrote = writesWhileHeld(t, locks, files, files.Home)

			return errSlackRefused
		}

		// Act
		_, _, err := config.SaveEdit(config.Edit{
			Files: files, Locks: locks, Read: read, Over: revision, Edited: edited,
			PlaceSlackCredentials: refusedWhileAnotherSaves,
		})

		// Assert
		writeErr := holderWrote()
		if !errors.Is(err, errSlackRefused) || !errors.Is(err, config.ErrChangedOnDisk) || writeErr != nil ||
			!holdsHeldFile(t, files.Home) {
			t.Errorf("SaveEdit = %v (the holder's write: %v); want Slack's refusal and %v, and the holder's write "+
				"kept: taking a write back checks the revision only once it holds the lock", err, writeErr,
				config.ErrChangedOnDisk)
		}
	})
}
