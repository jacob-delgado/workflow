// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestSaveOverIntoADirectoryItCannotWriteKeepsThePreviousFile(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root writes into a directory whatever its mode, so the save would succeed")
	}

	dir := t.TempDir()
	path := write(t, dir, readContents)
	read := revisionOf(t, path)
	sealDirectory(t, dir)

	// Act
	_, err := config.SaveOver(path, savedConfig(), read)

	// Assert
	if err == nil {
		t.Error("SaveOver into a directory it cannot write = nil, want the failure")
	}

	if got := fileContents(t, path); got != readContents {
		t.Errorf("the file holds %q, want the previous contents, %q", got, readContents)
	}
}

func TestSaveOverReplacesTheFileRatherThanWritingIntoIt(t *testing.T) {
	t.Parallel()

	// Arrange
	path := write(t, t.TempDir(), readContents)
	read := revisionOf(t, path)
	before := lstat(t, path)

	// Act
	_, err := config.SaveOver(path, savedConfig(), read)
	// Assert
	if err != nil {
		t.Fatalf("SaveOver: %v", err)
	}

	if os.SameFile(before, lstat(t, path)) {
		t.Error("SaveOver wrote into the file it found, want a new file in its place")
	}
}

func TestSaveOverThroughALinkReplacesTheFileItPointsAt(t *testing.T) {
	t.Parallel()

	// A dotfiles manager keeps the file in a directory of its own and links it
	// in where the configuration is looked for, at times before it has written
	// the file the link names. Each case makes the link and returns its path and
	// the file it names.
	cases := map[string]func(t *testing.T) (path, target string){
		"an existing file": func(t *testing.T) (string, string) {
			t.Helper()
			target := write(t, t.TempDir(), readContents)

			return linkedFrom(t, t.TempDir(), target), target
		},
		"a file not yet written": func(t *testing.T) (string, string) {
			t.Helper()
			target := filepath.Join(t.TempDir(), "workflow.json")

			return linkedFrom(t, t.TempDir(), target), target
		},
		"a file not yet written, linked by a relative path": func(t *testing.T) (string, string) {
			t.Helper()
			dir := t.TempDir()
			mkdir(t, filepath.Join(dir, "dotfiles"))

			linked := filepath.Join("dotfiles", "workflow.json")

			return linkedFrom(t, dir, linked), filepath.Join(dir, linked)
		},
		// The link sits in a directory reached through another link, and its
		// ".." is taken from where the link really is, not from the path's text.
		"a file not yet written, linked relatively from a linked directory": func(t *testing.T) (string, string) {
			t.Helper()
			actual := t.TempDir()
			mkdir(t, filepath.Join(actual, "home"))
			mkdir(t, filepath.Join(actual, "dotfiles"))
			linkedFrom(t, filepath.Join(actual, "home"), filepath.Join("..", "dotfiles", "workflow.json"))
			alias := filepath.Join(t.TempDir(), "home")
			symlink(t, filepath.Join(actual, "home"), alias)

			return filepath.Join(alias, config.FileName), filepath.Join(actual, "dotfiles", "workflow.json")
		},
	}

	for name, place := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path, target := place(t)
			read := revisionOf(t, path)

			// Act
			_, err := config.SaveOver(path, savedConfig(), read)
			// Assert
			if err != nil {
				t.Fatalf("SaveOver: %v", err)
			}

			if mode := lstat(t, path).Mode(); mode&fs.ModeSymlink == 0 {
				t.Errorf("%s is %v after the save, want the link kept", path, mode)
			}

			saved, err := config.LoadFile(target)
			if err != nil || saved.Jira.BaseURL != savedURL {
				t.Errorf("the linked file reads back as Jira at %q (%v), want the saved configuration", saved.Jira.BaseURL, err)
			}
		})
	}
}

//nolint:paralleltest // t.Chdir moves the whole process, so these run serially.
func TestSaveToAnEmptyPathWritesNothing(t *testing.T) {
	cases := map[string]func(cfg config.Config) error{
		"Save": func(cfg config.Config) error {
			return config.Save("", cfg)
		},
		"SaveOver": func(cfg config.Config) error {
			_, err := config.SaveOver("", cfg, config.Revision{})

			return err
		},
	}

	for name, save := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			// An empty path names no file, so nothing may be written to the
			// directory the process runs in, which is what "" resolves against.
			dir := t.TempDir()
			t.Chdir(dir)

			// Act
			err := save(savedConfig())

			// Assert
			if !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s(\"\") = %v, want a refusal that names no file", name, err)
			}

			if names := entryNames(t, dir); len(names) != 0 {
				t.Errorf("the working directory holds %q after the save, want nothing", names)
			}
		})
	}
}

func TestSaveThatFailsLeavesNoFileBesideThePath(t *testing.T) {
	t.Parallel()

	cases := map[string]func(t *testing.T, path string){
		// The replacement is written, and renaming it over a directory fails.
		"a directory in its place": func(t *testing.T, path string) {
			t.Helper()
			mkdir(t, path)
		},
		// Following the link is what fails, before anything is written.
		"a link to itself": func(t *testing.T, path string) {
			t.Helper()
			symlink(t, path, path)
		},
		// The link names itself again by way of a directory that does not
		// exist, spelled out rather than joined, which would clean the detour
		// away; the replacement is written, and renaming it there fails.
		"a link back to itself through a missing directory": func(t *testing.T, path string) {
			t.Helper()

			separator := string(filepath.Separator)
			symlink(t, "missing"+separator+".."+separator+config.FileName, path)
		},
	}

	for name, occupy := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			path := filepath.Join(dir, config.FileName)
			occupy(t, path)

			// Act
			err := config.Save(path, config.Default())

			// Assert
			if err == nil {
				t.Error("Save = nil, want the failure to replace the path")
			}

			if names := entryNames(t, dir); !slices.Equal(names, []string{config.FileName}) {
				t.Errorf("the directory holds %q after the failed save, want only %q", names, config.FileName)
			}
		})
	}
}

// sealDirectory leaves dir readable but closed to new files until the test
// ends, when it opens it again so the test's own cleanup can remove it.
func sealDirectory(t *testing.T, dir string) {
	t.Helper()

	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("sealing %s: %v", dir, err)
	}

	t.Cleanup(func() {
		err := os.Chmod(dir, 0o700)
		if err != nil {
			t.Errorf("unsealing %s: %v", dir, err)
		}
	})
}

// linkedFrom makes a link named FileName in dir to target and returns its path.
func linkedFrom(t *testing.T, dir, target string) string {
	t.Helper()

	path := filepath.Join(dir, config.FileName)
	symlink(t, target, path)

	return path
}

// mkdir makes the directory at path, open to its owner alone.
func mkdir(t *testing.T, path string) {
	t.Helper()

	err := os.Mkdir(path, 0o700)
	if err != nil {
		t.Fatalf("making %s: %v", path, err)
	}
}

// symlink makes link a symbolic link to target.
func symlink(t *testing.T, target, link string) {
	t.Helper()

	err := os.Symlink(target, link)
	if err != nil {
		t.Fatalf("linking %s to %s: %v", link, target, err)
	}
}

// lstat describes the file at path itself, never what a link there points at.
func lstat(t *testing.T, path string) fs.FileInfo {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", path, err)
	}

	return info
}

// entryNames is the names of what dir holds, hidden files included, in order.
func entryNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}
