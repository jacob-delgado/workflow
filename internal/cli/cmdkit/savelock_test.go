// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

func TestTheFirstRunWritesOnlyOnceAnotherWorkflowsSaveLetsGo(t *testing.T) {
	t.Parallel()

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

	writtenWhileHeld := make(chan error, 1)

	go func() {
		time.Sleep(time.Second)

		_, statErr := os.Lstat(layer)
		writtenWhileHeld <- statErr

		unlock()
	}()

	// Act
	_, err = conn.Deps.Settings.Setup.Write(t.Context(), setup.Request{Place: setup.Repository})

	// Assert
	statErr := <-writtenWhileHeld
	if err != nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Write = %v; the file as the lock was let go: %v; want it written only once another workflow's "+
			"save let go of the lock", err, statErr)
	}
}
