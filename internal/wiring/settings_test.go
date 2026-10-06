// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// madeCache makes the store's cache in dir, by remembering a scope there.
func madeCache(t *testing.T, dir string) {
	t.Helper()

	err := store.New(dir, false).RecordScope(t.Context(), "github.com/acme/api", "auth", time.Now())
	if err != nil {
		t.Fatalf("making the cache: %v", err)
	}
}

func TestLocalDataListsTheStoresFiles(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	madeCache(t, dir)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	listed, files, err := deps.Settings.LocalData()

	// Assert
	if err != nil || listed != dir || len(files) == 0 || files[0].Kind != store.DataCache {
		t.Errorf("LocalData() = %q, %+v, %v; want the cache listed in %q", listed, files, err, dir)
	}
}

func TestRemoveLocalDataRemovesTheCache(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	madeCache(t, dir)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	err := deps.Settings.RemoveLocalData(store.CleanCache)

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, "workflow.db"))
	if err != nil || !os.IsNotExist(statErr) {
		t.Errorf("RemoveLocalData = %v, and the cache is still there (%v); want it removed", err, statErr)
	}
}

func TestLocalDataWithNoStoreDirectoryIsAnError(t *testing.T) {
	// Arrange
	t.Setenv("HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	_, _, err := deps.Settings.LocalData()

	// Assert
	if !errors.Is(err, store.ErrNoDir) {
		t.Errorf("LocalData = %v, want store.ErrNoDir", err)
	}
}

func TestRemoveLocalDataWithNoStoreDirectoryIsAnError(t *testing.T) {
	// Arrange
	t.Setenv("HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	err := deps.Settings.RemoveLocalData(store.CleanCache)

	// Assert
	if !errors.Is(err, store.ErrNoDir) {
		t.Errorf("RemoveLocalData = %v, want store.ErrNoDir", err)
	}
}

func TestLocalDataThatCannotBeReadIsAnError(t *testing.T) {
	// Arrange
	// The state directory is a plain file, so nothing can be looked up under it.
	notADir := filepath.Join(t.TempDir(), "state")

	err := os.WriteFile(notADir, nil, 0o600)
	if err != nil {
		t.Fatalf("writing the file: %v", err)
	}

	t.Setenv("XDG_STATE_HOME", notADir)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	_, files, err := deps.Settings.LocalData()

	// Assert
	if err == nil || files != nil {
		t.Errorf("LocalData() = %+v, %v; want an error and no files", files, err)
	}
}

func TestRemoveLocalDataRefusesWhatIsNotTheStoresOwnFile(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)

	err := os.MkdirAll(filepath.Join(dir, "workflow.db"), 0o700)
	if err != nil {
		t.Fatalf("making a directory where the cache belongs: %v", err)
	}

	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	err = deps.Settings.RemoveLocalData(store.CleanCache)

	// Assert
	if !errors.Is(err, store.ErrCleanRefused) {
		t.Errorf("RemoveLocalData = %v, want store.ErrCleanRefused", err)
	}
}

// settingsToken is the Jira token in the file Settings reads.
const settingsToken = "jira-settings-token-7777"

// settingsFile writes a configuration holding settingsToken and wires the
// interface to it.
func settingsFile(t *testing.T) (string, tui.Deps) {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)

	err := os.WriteFile(path, []byte(`{"jira": {"base_url": "https://jira.example.com", "token": "`+
		settingsToken+`", "project": "PROJ"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("reading the configuration: %v", err)
	}

	return path, wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)
}

func TestSettingsReadNeverHandsTheInterfaceACredential(t *testing.T) {
	// Arrange
	_, deps := settingsFile(t)

	// Act
	read, _, err := deps.Settings.Read()

	// Assert
	if err != nil || read.Jira.Token.Reveal() == settingsToken || read.Jira.Project != "PROJ" {
		t.Errorf("Read() = project %q, token unmasked %t, %v; want the token masked",
			read.Jira.Project, read.Jira.Token.Reveal() == settingsToken, err)
	}
}

func TestSettingsSaveKeepsTheTokenItWasShownMasked(t *testing.T) {
	// Arrange
	path, deps := settingsFile(t)
	read, over, _ := deps.Settings.Read()
	read.Jira.Project = "OSS"

	// Act
	saved, _, err := deps.Settings.Save(read, over)

	// Assert
	onDisk, _, readErr := config.LoadLayersAt(config.Files{Home: path})
	if err != nil || readErr != nil || onDisk.Jira.Project != "OSS" || onDisk.Jira.Token.Reveal() != settingsToken {
		t.Errorf("saved project %q, token kept %t (%v, %v); want the edit saved and the token kept",
			onDisk.Jira.Project, onDisk.Jira.Token.Reveal() == settingsToken, err, readErr)
	}

	if saved.Jira.Token.Reveal() == settingsToken {
		t.Errorf("Save answered the token unmasked")
	}
}

func TestSettingsSaveRefusesAFileChangedSinceTheRead(t *testing.T) {
	// Arrange
	path, deps := settingsFile(t)
	read, over, _ := deps.Settings.Read()

	err := os.WriteFile(path, []byte(`{"jira": {"project": "ELSE"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("changing the file: %v", err)
	}

	// Act
	_, _, err = deps.Settings.Save(read, over)

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) {
		t.Errorf("Save over a changed file = %v, want ErrChangedOnDisk", err)
	}
}

func TestSettingsSaveRefusesARevisionItDidNotRead(t *testing.T) {
	// Arrange
	_, deps := settingsFile(t)
	read, _, _ := deps.Settings.Read()

	// Act
	_, _, err := deps.Settings.Save(read, config.Revision{})

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) {
		t.Errorf("Save over an unread revision = %v, want ErrChangedOnDisk", err)
	}
}

func TestSettingsReadRefusesAFileThatIsNotValid(t *testing.T) {
	// Arrange
	path, deps := settingsFile(t)

	err := os.WriteFile(path, []byte(`{"jira": {"nonsense": 1}}`), config.FileMode)
	if err != nil {
		t.Fatalf("breaking the file: %v", err)
	}

	// Act
	_, _, err = deps.Settings.Read()

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Read() = %v, want ErrInvalid", err)
	}
}
