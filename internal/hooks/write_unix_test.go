// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package hooks_test

// A write that stops part-way leaves nothing behind, not even the part of the
// file it was writing when it stopped, so a lefthook.yml cut short never blocks
// the offer from being tried again. A limit on the size of the files the process
// may write is what stops it here: it holds for every write the process makes,
// so these tests run alone and lift it the moment Write returns.
//
// The limit is far past the size of anything else the process writes meanwhile,
// go test's own record of the files a test opens among them: at a byte, a flush
// of that record while the limit held would fail the whole run.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// fileSizeLimit is the size no file the process writes may pass while a test
// holds the limit.
const fileSizeLimit = 1 << 20

// limitFileSize caps the size of every file the process writes at
// fileSizeLimit, and returns what lifts the cap again, which the test also runs
// at its end.
func limitFileSize(t *testing.T) func() {
	t.Helper()

	var was syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &was)
	if err != nil {
		t.Fatalf("reading the file size limit: %v", err)
	}

	lift := func() {
		err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &was)
		if err != nil {
			t.Fatalf("lifting the file size limit: %v", err)
		}
	}

	t.Cleanup(lift)

	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: fileSizeLimit, Max: was.Max})
	if err != nil {
		t.Fatalf("limiting the file size: %v", err)
	}

	return lift
}

// pastTheLimit is text one byte longer than a file may be under the limit.
func pastTheLimit() string {
	return strings.Repeat("#", fileSizeLimit+1)
}

// shortConfig is a configuration that fits under the limit.
const shortConfig = "pre-commit:\n  commands: {}\n"

// filesIn is every file below dir, by its path within dir.
func filesIn(t *testing.T, dir string) []string {
	t.Helper()

	var files []string

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		files = append(files, strings.TrimPrefix(path, dir))

		return nil
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	return files
}

//nolint:paralleltest // the file size limit holds for every write the process makes, so these run alone.
func TestAWriteCutShortLeavesNoFileBehind(t *testing.T) {
	script := hooks.File{Path: ".lefthook/" + preCommit + "/" + preCommit, Contents: pastTheLimit()}

	cases := map[string]hooks.Generated{
		"in the configuration": {Config: pastTheLimit()},
		"in a script":          {Config: shortConfig, Scripts: []hooks.File{script}},
	}

	for name, generated := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			lift := limitFileSize(t)

			// Act
			err := hooks.Write(dir, generated)

			lift()

			// Assert
			if !errors.Is(err, syscall.EFBIG) {
				t.Errorf("Write = %v, want the write's own failure", err)
			}

			if left := filesIn(t, dir); len(left) != 0 {
				t.Errorf("a write cut short left %q behind, want nothing", left)
			}
		})
	}
}

//nolint:paralleltest // the file size limit holds for every write the process makes, so this runs alone.
func TestAWriteCutShortCanBeTriedAgain(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	generated := hooks.Generated{Config: pastTheLimit()}
	lift := limitFileSize(t)

	err := hooks.Write(dir, generated)

	lift()

	if err == nil {
		t.Fatal("Write under the limit succeeded, want it cut short")
	}

	// Act
	err = hooks.Write(dir, generated)

	// Assert
	config, readErr := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if err != nil || readErr != nil || string(config) != generated.Config {
		t.Errorf("the second Write = %v, and lefthook.yml holds %d bytes (%v); want the configuration written whole",
			err, len(config), readErr)
	}
}
