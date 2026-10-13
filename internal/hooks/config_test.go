// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

func TestRunCommandRunsOneHookWithoutATerminal(t *testing.T) {
	t.Parallel()

	// Act
	command := hooks.RunCommand("/work", preCommit)

	// Assert
	if command.Dir != "/work" || command.Name != "lefthook" ||
		!slices.Equal(command.Args, []string{"run", preCommit, "--no-tty"}) {
		t.Errorf("RunCommand = %+v", command)
	}
}

func TestInstallCommandInstallsInTheRepositoryRoot(t *testing.T) {
	t.Parallel()

	// Act
	command := hooks.InstallCommand("/work")

	// Assert
	if command.Dir != "/work" || command.Name != "lefthook" || !slices.Equal(command.Args, []string{"install"}) {
		t.Errorf("InstallCommand = %+v", command)
	}
}

func TestHasConfigRecognizesEveryNameLefthookReads(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"lefthook.yml", ".lefthook.yml", "lefthook.yaml", ".lefthook.yaml", "lefthook.toml", ".lefthook.toml",
		"lefthook.json", ".lefthook.json", "lefthook.jsonc", ".lefthook.jsonc", ".config/lefthook.yml",
		".config/lefthook.jsonc",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if !hooks.HasConfig(fstest.MapFS{name: {Data: []byte("x")}}) {
				t.Errorf("HasConfig did not see %s", name)
			}
		})
	}
}

func TestHasConfigSeesNoConfigurationWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	repository := fstest.MapFS{"lefthook-local.yml": {Data: []byte("x")}, "README.md": {Data: []byte("x")}}

	// Act & Assert
	if hooks.HasConfig(repository) {
		t.Error("HasConfig saw a configuration in a repository without one")
	}
}
