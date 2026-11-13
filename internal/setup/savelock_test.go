// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup_test

// A file written over a home file holds the save locks Where carries, so a
// save another workflow makes meanwhile is waited for, never written over.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// overAHomeFile is a Where whose home directory holds a file, and whose
// save locks, kept in a state directory of the test's own, are held for the
// repository's file over it as another workflow saving it would hold them,
// until the test ends.
func overAHomeFile(t *testing.T) (setup.Where, config.Files) {
	t.Helper()

	home := t.TempDir()

	err := os.WriteFile(filepath.Join(home, config.FileName), []byte(`{"jira": {"project": "OSS"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	where := setup.Where{WorkDir: t.TempDir(), HomeDir: home, Locks: config.SaveLocksIn(t.TempDir())}
	layers := where.Layers(setup.Repository)

	unlock, err := where.Locks.Hold(layers)
	if err != nil {
		t.Fatalf("holding the save locks: %v", err)
	}

	t.Cleanup(unlock)

	return where, layers
}

func TestCreatingAFileOverTheHomeFileWaitsOutAnotherWorkflowsSave(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		where, layers := overAHomeFile(t)

		_, over, err := config.LoadLayersAt(layers)
		if err != nil {
			t.Fatalf("reading the layers: %v", err)
		}

		// Act
		err = where.Create(layers.Repo, layers, config.Default(), over)

		// Assert
		_, statErr := os.Lstat(layers.Repo)
		if !errors.Is(err, config.ErrSaveLocked) || !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("Create = %v, the file: %v; want %v and no file made", err, statErr, config.ErrSaveLocked)
		}
	})
}

func TestSavingOverTheHomeFileWaitsOutAnotherWorkflowsSave(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		where, layers := overAHomeFile(t)

		_, over, err := config.LoadLayersAt(layers)
		if err != nil {
			t.Fatalf("reading the layers: %v", err)
		}

		// Act
		err = where.Save(setup.Repository, config.Default(), over)

		// Assert
		_, statErr := os.Lstat(layers.Repo)
		if !errors.Is(err, config.ErrSaveLocked) || !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("Save = %v, the file: %v; want %v and no file written", err, statErr, config.ErrSaveLocked)
		}
	})
}
