// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestConfigInitOverAHomeFileSavesUnderTheSaveLocksOfTheStoresDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.HomeAndRepository(t)

	stateDir, err := clitest.EnvironmentFor(t, where).Process.StateDir()
	if err != nil {
		t.Fatalf("finding the store's directory: %v", err)
	}

	// Act
	_, err = clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "config", "init", "--template")

	// Assert
	locks, globErr := filepath.Glob(filepath.Join(stateDir, "config-*.lock"))
	if err != nil || globErr != nil || len(locks) == 0 {
		t.Errorf("config init = %v; the store's directory holds the save locks %v (%v); want the layer written "+
			"under a save lock kept there", err, locks, globErr)
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
