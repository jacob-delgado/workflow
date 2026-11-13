// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

func TestTheFirstRunWritesNothingWhileAnotherWorkflowHoldsTheSaveLock(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		home, dir := t.TempDir(), t.TempDir()
		clitest.WriteFile(t, home, `{"jira": {"project": "OSS"}}`)
		env := clitest.EnvironmentFor(t, clitest.Place{Dir: dir, Home: home})
		layer := filepath.Join(dir, config.FileName)

		var conn cmdkit.Connection

		root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
			conn = cmdkit.ConnectAt(cmd.Context(), cmd, dir, nil).WithSetup(cmd, nil)

			return nil
		})

		_, err := runTree(t, root, probeCommand)
		if err != nil {
			t.Fatalf("connecting: %v", err)
		}

		unlock, err := env.Process.SaveLocks().Hold(config.Files{Home: filepath.Join(home, config.FileName), Repo: layer})
		if err != nil {
			t.Fatalf("holding the save lock: %v", err)
		}

		defer unlock()

		// Act
		_, err = conn.Deps.Settings.Setup.Write(t.Context(), setup.Request{Place: setup.Repository})

		// Assert
		_, statErr := os.Lstat(layer)
		if !errors.Is(err, config.ErrSaveLocked) || !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("Write = %v; the file: %v; want %v and no file written while another workflow held the "+
				"save lock", err, statErr, config.ErrSaveLocked)
		}
	})
}
