// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// Every save of the configuration the wiring makes holds the save locks kept
// in the store's directory, so a save another workflow makes meanwhile is
// waited for, never written over, and no lock lands in a working tree.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// savedElsewhere is a home file holding contents, read, in an environment
// whose store's directory is the test's own, with the file's save lock held
// there as another workflow saving it would hold it, until the test ends.
func savedElsewhere(t *testing.T, contents string) (wiring.Environment, config.Config, config.Revision) {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, over, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("reading the configuration: %v", err)
	}

	env := stateIn(stateDirAt(t.TempDir()))

	unlock, err := env.SaveLocks().Hold(cfg.Layers())
	if err != nil {
		t.Fatalf("holding the save lock: %v", err)
	}

	t.Cleanup(unlock)

	return env, cfg, over
}

func TestTheTerminalsSettingsSaveWaitsOutAnotherWorkflowsSave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Arrange
		env, cfg, over := savedElsewhere(t, `{"jira": {"project": "PROJ"}}`)
		deps, _ := env.Deps(cfg, onOrigin(t.TempDir(), ""), nil)

		_, _, err := deps.Settings.Read()
		if err != nil {
			t.Fatalf("reading Settings: %v", err)
		}

		edited := cfg
		edited.Jira.Project = "EDITED"

		// Act
		_, _, err = deps.Settings.Save(t.Context(), edited, nil, over)

		// Assert
		onDisk, _, readErr := config.LoadLayersAt(cfg.Layers())
		if !errors.Is(err, config.ErrSaveLocked) || readErr != nil || onDisk.Jira.Project != "PROJ" {
			t.Errorf("Save = %v; the file names %q (%v); want %v and the file as it was", err,
				onDisk.Jira.Project, readErr, config.ErrSaveLocked)
		}
	})
}

func TestASlackPairTheFileKeepsWaitsOutAnotherWorkflowsSave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Arrange
		env, cfg, over := savedElsewhere(t, `{"messaging": {"kind": "slack", "client_id": "1234.5678", `+
			`"channel": "#dev", "client_secret": "kept-secret", "refresh_token": "xoxe-1-kept"}}`)

		// Act
		err := env.SlackStore(cfg).Save(t.Context(), slackauth.Credentials{
			ClientSecret: "kept-secret", RefreshToken: "xoxe-1-renewed", AccessToken: "xoxp-renewed",
		})

		// Assert
		now, revErr := config.RevisionOfLayers(cfg.Layers())
		if !errors.Is(err, config.ErrSaveLocked) || revErr != nil || now != over {
			t.Errorf("Save = %v; the file is as it was: %t (%v); want %v and no pair written", err, now == over,
				revErr, config.ErrSaveLocked)
		}
	})
}

func TestWithNoStoreDirectoryASaveTakesNoLock(t *testing.T) {
	// Arrange
	env := stateIn(func() (string, error) { return "", errNoStateDir })

	// Act
	locks := env.SaveLocks()

	// Assert
	if locks != config.SaveLocksIn("") {
		t.Errorf("SaveLocks() = %+v; want the locks of no directory, which a save takes none of", locks)
	}
}
