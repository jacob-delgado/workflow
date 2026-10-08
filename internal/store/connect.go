// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
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

// openDatabase is the database file at path for writing, with the
// shared-access pragmas, made where there is none.
func openDatabase(path string) *sql.DB {
	return sqlitefile.Open(path, fmt.Sprintf(dsnPragmas, busyTimeoutMillis))
}

// openAsItIs is the database named, already on disk, for reading alone: it
// makes no directory, prepares no schema, narrows no mode and writes no row.
// Like any reader of a write-ahead-logged database, SQLite may leave the log's
// two companion files beside it, owner-only, until the next live open clears
// them.
func (s Store) openAsItIs(name string) *sql.DB {
	return sqlitefile.Open(filepath.Join(s.dir, name), fmt.Sprintf(readOnlyPragmas, busyTimeoutMillis))
}
