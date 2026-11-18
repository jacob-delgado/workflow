// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/jacob-delgado/workflow/internal/sqlitefile"
)

// busyTimeoutMillis is how long a write waits for another connection's lock
// before giving up, so the interface and the web server sharing the file do not
// fail on momentary contention.
const busyTimeoutMillis = 5000

// dsnPragmas turns on write-ahead logging and the busy timeout for every
// connection, which is what lets two processes share the one file, and foreign
// keys, which SQLite enforces per-connection so an ON DELETE CASCADE only fires
// when it is on.
const dsnPragmas = "_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

// readOnlyPragmas opens the database file for reading only, never creating it,
// with the same busy timeout, so a read waits out another connection's lock
// rather than failing.
const readOnlyPragmas = "mode=ro&_pragma=busy_timeout(%d)"

// storeFile is one of the store's databases: its name in the store
// directory, the pragmas its writing connections open with, the schema
// version this build stamps into it, and the statements that make its tables
// where they are missing.
type storeFile struct {
	name    string
	pragmas string
	version int
	tables  func() []string
}

// openDatabase is the database file at path for writing, with pragmas, made
// where there is none.
func openDatabase(path, pragmas string) *sql.DB {
	return sqlitefile.Open(path, fmt.Sprintf(pragmas, busyTimeoutMillis))
}

// openFile makes the store directory, opens the file at this build's schema
// version, discarding one another build left, makes the tables it is
// missing, and restricts the file to its owner. Two processes making a fresh
// file both stamp it and both run its statements, which is why every one of
// them is IF NOT EXISTS.
func (s Store) openFile(ctx context.Context, file storeFile) (*sql.DB, error) {
	err := s.makeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, file.name)

	// The file is made, stamped and given its tables whatever becomes of the
	// caller, so a stamp or a removal still fails only when the file changes
	// underneath it, and never for a caller who left.
	making := context.WithoutCancel(ctx)

	database, err := openCurrent(making, path, file)
	if err != nil {
		return nil, err
	}

	err = makeTables(making, database, file)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	// The open created the file honoring the umask; narrow it now that it
	// exists, so the database is readable only by its owner.
	restrictToOwner(path, filePerm)

	return database, nil
}

// readFile opens the file for a read, and reports false when there is
// nothing to read: a store that is off, or a read-only one with no such file
// or one at another schema version. A dry run reads a file at another
// version as empty, since the live open would remake it: what it holds is
// not what a run reads, and its tables need not be this build's.
func (s Store) readFile(ctx context.Context, file storeFile) (*sql.DB, bool, error) {
	if s.nothingToReadIn(file.name) {
		return nil, false, nil
	}

	if s.mode != ModeReadOnly {
		database, err := s.openFile(ctx, file)

		return database, err == nil, err
	}

	database := s.openAsItIs(file.name)

	state, err := readState(ctx, database)
	if err != nil || state.version != file.version {
		_ = database.Close()

		return nil, false, err
	}

	return database, true, nil
}

// makeTables runs every statement that makes the file's tables where they
// are missing. None alters a table: a file at another version was discarded
// before this ran, so what it meets is this version's schema or nothing.
func makeTables(ctx context.Context, database *sql.DB, file storeFile) error {
	for _, statement := range file.tables() {
		_, err := database.ExecContext(ctx, statement)
		if err != nil {
			return fmt.Errorf("preparing the store schema: %w", err)
		}
	}

	return nil
}

// openAsItIs is the database named, already on disk, for reading alone: it
// makes no directory, prepares no schema, narrows no mode and writes no row.
// Like any reader of a write-ahead-logged database, SQLite may leave the log's
// two companion files beside it, owner-only, until the next live open clears
// them.
func (s Store) openAsItIs(name string) *sql.DB {
	return sqlitefile.Open(filepath.Join(s.dir, name), fmt.Sprintf(readOnlyPragmas, busyTimeoutMillis))
}
