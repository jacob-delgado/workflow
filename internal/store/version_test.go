// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The schema has one version, stamped into the file as SQLite's user_version. A
// file at another version — an older build's, or one someone changed — is
// discarded whole and made again, since nothing the store keeps is worth
// carrying over; a read-only store reads whatever file it finds as it is.

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// currentVersion is the schema version this build stamps; it moves with the
// store's own constant.
const currentVersion = 1

// witness is an application_id the store never writes, so a file that still
// carries it is the file that was there, not one made in its place.
const witness = 42

// stampVersion sets the file's user_version, as a build with another schema
// would have.
func stampVersion(t *testing.T, dir string, version int) {
	t.Helper()

	seedDatabaseAt(t, dir, version)
}

// readPragma reads one integer-valued pragma from the file.
func readPragma(t *testing.T, dir, name string) int {
	t.Helper()

	database := openFile(t, filepath.Join(dir, "workflow.db"))
	defer func() { _ = database.Close() }()

	var value int

	err := database.QueryRowContext(t.Context(), "PRAGMA "+name).Scan(&value)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return value
}

func TestAStoreAtAnotherSchemaVersionIsStartedFresh(t *testing.T) {
	t.Parallel()

	cases := map[string]int{
		"an older build's file, never stamped": 0,
		"a later build's file":                 currentVersion + 1,
	}

	for name, version := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			kept := store.New(dir, false)

			err := kept.RecordScope(t.Context(), repo, recorded, theTime())
			if err != nil {
				t.Fatalf("keeping a scope: %v", err)
			}

			stampVersion(t, dir, version)

			// Act
			scope, found, err := kept.LastScope(t.Context(), repo)

			// Assert
			if err != nil || found || scope != "" {
				t.Errorf("LastScope = %q, %v, %v; want nothing found and no error from a file started fresh",
					scope, found, err)
			}

			if got := readPragma(t, dir, "user_version"); got != currentVersion {
				t.Errorf("the file is at version %d, want %d", got, currentVersion)
			}
		})
	}
}

func TestAFreshStoreIsStampedWithItsSchemaVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	err := store.New(dir, false).RecordScope(t.Context(), repo, recorded, theTime())
	// Assert
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	if got := readPragma(t, dir, "user_version"); got != currentVersion {
		t.Errorf("a fresh store is at version %d, want %d", got, currentVersion)
	}
}

func TestAFileAtAnotherVersionWithNoTablesIsKeptAndStamped(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file stamped with another version but holding no table is not another
	// build's store, so it is kept and stamped rather than made again; the
	// application_id, which the store never writes, tells the two apart.
	dir := t.TempDir()
	seedDatabaseAt(t, dir, currentVersion+1, statement{query: fmt.Sprintf("PRAGMA application_id = %d", witness)})

	// Act
	err := store.New(dir, false).RecordScope(t.Context(), repo, recorded, theTime())
	// Assert
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	if got := readPragma(t, dir, "user_version"); got != currentVersion {
		t.Errorf("the file is at version %d, want %d", got, currentVersion)
	}

	if got := readPragma(t, dir, "application_id"); got != witness {
		t.Errorf("the file's application_id is %d, want the %d it carried: the same file, not a new one", got, witness)
	}
}

func TestAReadOnlyStoreReadsAStoreAtAnotherSchemaVersionAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.db")

	err := store.New(dir, false).RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	stampVersion(t, dir, currentVersion+1)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	scope, found, err := store.New(dir, false).ReadOnly().LastScope(t.Context(), repo)

	// Assert
	if err != nil || !found || scope != recorded {
		t.Errorf("LastScope = %q, %v, %v; want the kept scope read as it is", scope, found, err)
	}

	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("a read-only read changed the file (err %v), want it left as it was", err)
	}

	if got := readPragma(t, dir, "user_version"); got != currentVersion+1 {
		t.Errorf("a read-only read left the file at version %d, want %d untouched", got, currentVersion+1)
	}
}

func TestAReadOfAStoreAtThisVersionWritesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The version is stamped by the open that makes the file, not by every
	// open, so a read stays a read: it takes no write lock and moves no byte.
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.db")
	kept := store.New(dir, false)

	err := kept.RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	scope, found, err := kept.LastScope(t.Context(), repo)

	// Assert
	if err != nil || !found || scope != recorded {
		t.Fatalf("LastScope = %q, %v, %v; want the kept scope", scope, found, err)
	}

	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("a read changed the file (err %v), want it left as it was", err)
	}
}

// heldOpenWithARow is a connection another program keeps on the store's file,
// with a row of its own written through it, so the file's write-ahead log holds
// something that program still expects to read.
func heldOpenWithARow(t *testing.T, dir string) *sql.Conn {
	t.Helper()

	holder := openFile(t, filepath.Join(dir, "workflow.db")+"?_pragma=journal_mode(WAL)")
	t.Cleanup(func() { _ = holder.Close() })

	held, err := holder.Conn(t.Context())
	if err != nil {
		t.Fatalf("holding a connection: %v", err)
	}

	t.Cleanup(func() { _ = held.Close() })

	_, err = held.ExecContext(t.Context(),
		`INSERT INTO scopes (repo, scope, updated_at) VALUES (?, ?, ?)`, "other", "held", "2026-09-29T12:00:00Z")
	if err != nil {
		t.Fatalf("writing as another program: %v", err)
	}

	return held
}

func TestADiscardLeavesAProcessStillOnTheOldFileItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses to remove a file another program holds open")
	}

	// Another program still holds the old file open, with a row of its own in
	// its write-ahead log. Discarding the file with its -wal and -shm leaves
	// that program on what it opened, rather than on a log the fresh file then
	// resets under it.
	dir := t.TempDir()
	kept := store.New(dir, false)

	err := kept.RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("keeping a scope: %v", err)
	}

	stampVersion(t, dir, currentVersion+1)
	held := heldOpenWithARow(t, dir)

	// Act
	_, found, err := kept.LastScope(t.Context(), repo)

	// Assert
	if err != nil || found {
		t.Errorf("LastScope found %v, err %v; want nothing found and no error from a file started fresh", found, err)
	}

	if got := readPragma(t, dir, "user_version"); got != currentVersion {
		t.Errorf("the file is at version %d, want %d", got, currentVersion)
	}

	var rows int

	err = held.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM scopes`).Scan(&rows)
	if err != nil || rows != 2 {
		t.Errorf("the other program reads %d rows (err %v), want its 2 still there on the file it holds", rows, err)
	}
}
