// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

// heldFor is how long the test holds the save lock as another workflow
// would: long enough that a write not waiting on it lands before the lock is
// let go.
const heldFor = time.Second

func TestConfigInitOverAHomeFileWaitsOutAnotherWorkflowsSave(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.HomeAndRepository(t)
	layer := filepath.Join(where.Dir, config.FileName)
	files := config.Files{Home: filepath.Join(where.Home, config.FileName), Repo: layer}

	unlock, err := clitest.EnvironmentFor(t, where).Process.SaveLocks().Hold(files)
	if err != nil {
		t.Fatalf("holding the save lock: %v", err)
	}

	writtenWhileHeld := make(chan error, 1)

	go func() {
		time.Sleep(heldFor)

		_, statErr := os.Lstat(layer)
		writtenWhileHeld <- statErr

		unlock()
	}()

	// Act
	_, err = clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "config", "init", "--template")

	// Assert
	statErr := <-writtenWhileHeld
	if err != nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("config init = %v; the layer as the lock was let go: %v; want it written only once another "+
			"workflow's save let go of the lock", err, statErr)
	}
}

func TestTheWebServersSettingsSaveUnderTheSaveLocksOfTheStoresDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}

	// Act
	ran := runRootSwitching(t, where, nil, "--web")

	// Assert
	want := clitest.EnvironmentFor(t, where).Process.SaveLocks()
	if ran.err != nil || ran.deps.SaveLocks != want {
		t.Errorf("workflow --web = %v, saving under %+v; want %+v, the store's directory", ran.err, ran.deps.SaveLocks,
			want)
	}
}
