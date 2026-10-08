// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// subdirectory makes the directory names below parent, and returns it.
func subdirectory(t *testing.T, parent string, names ...string) string {
	t.Helper()

	dir := filepath.Join(append([]string{parent}, names...)...)

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		t.Fatalf("making %s: %v", dir, err)
	}

	return dir
}

// markRepository makes dir the root of a repository, as its .git does.
func markRepository(t *testing.T, dir string) {
	t.Helper()

	err := os.Mkdir(filepath.Join(dir, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}
}

func TestOutsideARepositoryLocateReadsNoFileInAParentDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// A parent directory any local user could have written to, such as a shared
	// /tmp, is no repository, so its file belongs to nothing workflow runs in.
	parent := t.TempDir()
	write(t, parent, completeConfig)
	workDir := subdirectory(t, parent, "sub")

	// Act
	files, err := config.Locate(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("Locate = %+v, %v; want ErrNotFound, the parent's file unread", files, err)
	}
}

func TestOutsideARepositoryLocateReadsTheFileInTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	want := write(t, workDir, completeConfig)

	// Act
	files, err := config.Locate(workDir, t.TempDir())

	// Assert
	if err != nil || files != (config.Files{Repo: want}) {
		t.Errorf("Locate = %+v, %v; want the working directory's own file", files, err)
	}
}

func TestTheHomeFileFoundFromWithinIsStillTheHomeFile(t *testing.T) {
	t.Parallel()

	cases := map[string]func(t *testing.T, home string) string{
		"the home directory itself": func(_ *testing.T, home string) string { return home },
		"a repository at the home directory": func(t *testing.T, home string) string {
			t.Helper()

			markRepository(t, home)

			return subdirectory(t, home, "dotfiles")
		},
	}

	for name, workDirIn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			home := t.TempDir()
			homeFile := write(t, home, completeConfig)
			workDir := workDirIn(t, home)

			// Act
			files, err := config.Locate(workDir, home)

			// Assert
			if err != nil || files != (config.Files{Home: homeFile}) {
				t.Errorf("Locate = %+v, %v; want the home file as the home file", files, err)
			}
		})
	}
}
