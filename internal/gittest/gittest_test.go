// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gittest_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

func TestLocalVariablesAreTheOnesGitNames(t *testing.T) {
	t.Parallel()

	// Act
	named := strings.Fields(gittest.Run(t, t.TempDir(), "rev-parse", "--local-env-vars"))

	// Assert
	// A git that adds one fails here, rather than leaving it set for a test
	// to inherit from a hook.
	listed := gittest.LocalVariables()
	if !slices.Equal(slices.Sorted(slices.Values(listed)), slices.Sorted(slices.Values(named))) {
		t.Errorf("LocalVariables = %q, want git's own list %q", listed, named)
	}
}

//nolint:paralleltest // it sets and clears the process's own environment, so it runs alone.
func TestClearLocalVariablesUnsetsEachOne(t *testing.T) {
	// Arrange
	for _, name := range gittest.LocalVariables() {
		t.Setenv(name, "/somewhere/else")
	}

	// Act
	err := gittest.ClearLocalVariables()
	// Assert
	if err != nil {
		t.Fatalf("ClearLocalVariables = %v", err)
	}

	for _, name := range gittest.LocalVariables() {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set, to %q", name, value)
		}
	}
}

func TestRunKeepsTheDevelopersConfigurationOut(t *testing.T) {
	// Arrange
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Someone Else\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	name := gittest.Run(t, t.TempDir(), "config", "--default", "nobody", "user.name")

	// Assert
	if name != "nobody" {
		t.Errorf("git read user.name %q, want the developer's own configuration kept out", name)
	}
}

func TestHookEnvironmentNamesTheGitDirectoryAndIndexOfTheRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := filepath.Join("repos", "api")

	// Act
	environment := gittest.HookEnvironment(repo)

	// Assert
	want := []string{
		"GIT_DIR=" + filepath.Join(repo, ".git"), "GIT_INDEX_FILE=" + filepath.Join(repo, ".git", "index"),
	}
	if !slices.Equal(environment, want) {
		t.Errorf("HookEnvironment = %q, want %q", environment, want)
	}
}

func TestChangedFilesNamesWhatWasAddedRemovedOrRewritten(t *testing.T) {
	t.Parallel()

	// Arrange
	const rewritten = "rewritten"

	dir := t.TempDir()

	for name, contents := range map[string]string{"kept": "same", rewritten: "before", "removed": "gone"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	before := gittest.FilesUnder(t, dir)

	for name, contents := range map[string]string{rewritten: "after", "added": "new"} {
		err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := os.Remove(filepath.Join(dir, "removed"))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	changed := gittest.ChangedFiles(before, gittest.FilesUnder(t, dir))

	// Assert
	if want := []string{"added", "removed", rewritten}; !slices.Equal(changed, want) {
		t.Errorf("ChangedFiles = %q, want %q", changed, want)
	}
}
