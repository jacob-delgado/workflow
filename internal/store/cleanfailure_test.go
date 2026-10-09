// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// Listing and cleaning the store's files meets what it finds in its
// directory: a path that is no directory, a table that cannot be counted,
// and, for a user other than root, a directory that will not let a file be
// renamed or removed.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// The cache's database and its write-ahead log, as a clean sets them aside.
const (
	cacheAside = "workflow.db.cleaning"
	walAside   = "workflow.db-wal.cleaning"
)

// notADirectory is a regular file where a store directory is named, so no
// file inside it can be looked at, whoever asks.
func notADirectory(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "not-a-directory")

	err := os.WriteFile(path, []byte("a file"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

// sealed leaves dir readable but closed to a rename or removal inside it, and
// opens it again as the test ends so its files can be removed.
func sealed(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root renames and removes in a directory whatever its mode")
	}

	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("sealing %s: %v", dir, err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

func TestFilesReportsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := notADirectory(t)

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	if !errors.Is(err, syscall.ENOTDIR) || len(files) != 0 {
		t.Errorf("Files = %+v, %v; want the directory it could not read reported and nothing listed", files, err)
	}
}

func TestATableThatCannotBeCountedIsLeftOutOfWhatItsFileHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	// The schema names a table of a kind no module of this build reads: it is
	// listed, so the file says it holds one, and it fails as it is counted.
	dir := t.TempDir()
	seedDatabase(t, dir,
		table(`CREATE TABLE announces (repo TEXT, pull INTEGER, moment INTEGER, announced_at TEXT)`),
		statement{query: `INSERT INTO announces (repo, pull, moment) VALUES (?, 42, 1)`, args: []any{repo}},
		statement{query: `PRAGMA writable_schema = ON`},
		statement{
			query: `INSERT INTO sqlite_master (type, name, tbl_name, rootpage, sql) VALUES (?, ?, ?, ?, ?)`,
			args:  []any{"table", "scopes", "scopes", 0, "CREATE VIRTUAL TABLE scopes USING no_such_module (repo)"},
		},
	)

	// Act
	files, err := store.Files(t.Context(), dir)

	// Assert
	if err != nil || len(files) != 1 || heldCount(files[0], "announcements") != 1 ||
		heldCount(files[0], "scopes") != -1 {
		t.Errorf("Files = %+v, %v; want the announcement counted and the scopes left out", files, err)
	}
}

func TestACleanReportsADirectoryItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := notADirectory(t)

	// Act
	err := store.Clean(dir, store.CleanAll)

	// Assert
	if !errors.Is(err, store.ErrNotCleaned) || !errors.Is(err, syscall.ENOTDIR) {
		t.Errorf("Clean = %v, want ErrNotCleaned for the directory it could not read", err)
	}

	if !exists(dir) {
		t.Errorf("%s was removed by a failed clean", dir)
	}
}

func TestACleanThatCannotRemoveWhatWasSetAsideReportsTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	leaveAside(t, dir, cacheAside, walAside)
	sealed(t, dir)

	// Act
	err := store.Clean(dir, store.CleanCache)

	// Assert
	if !errors.Is(err, store.ErrNotCleaned) || !strings.Contains(err.Error(), cacheAside) ||
		strings.Contains(err.Error(), walAside) {
		t.Errorf("Clean = %v, want ErrNotCleaned naming the first file it could not remove alone", err)
	}

	for _, name := range []string{cacheAside, walAside} {
		if !exists(filepath.Join(dir, name)) {
			t.Errorf("%s is gone, want it left where it could not be removed", name)
		}
	}
}

func TestACleanThatCannotSetAFileAsideRemovesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	keepBoth(t, dir)
	sealed(t, dir)

	// Act
	err := store.Clean(dir, store.CleanAll)

	// Assert
	if !errors.Is(err, store.ErrNotCleaned) {
		t.Errorf("Clean = %v, want ErrNotCleaned", err)
	}

	for _, path := range []string{filepath.Join(dir, cacheName), keptPath(dir)} {
		if !exists(path) {
			t.Errorf("%s was removed by a failed clean", path)
		}
	}
}
