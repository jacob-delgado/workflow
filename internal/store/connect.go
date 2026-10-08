// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"

	"modernc.org/sqlite" // the pure-Go SQLite driver, so CGO stays off
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

// connector opens each connection to the database dsn names with the pure-Go
// driver itself, so no lookup of a driver by name stands between the store and
// its file.
type connector struct {
	dsn string
}

var _ driver.Connector = connector{}

// Connect opens one connection to the database.
func (c connector) Connect(context.Context) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(c.dsn)
	if err != nil {
		return nil, fmt.Errorf("opening the store: %w", err)
	}

	return conn, nil
}

// Driver is the pure-Go SQLite driver.
func (connector) Driver() driver.Driver {
	return &sqlite.Driver{}
}

// connect is a handle on the database dsn names, which opens a connection
// only once it is first used.
func connect(dsn string) *sql.DB {
	return sql.OpenDB(connector{dsn: dsn})
}

// fileDSN is the address of the database file at path with params: a file
// URI, in which a percent sign escapes, a question mark starts the parameters
// and a hash ends the path, so the path escapes all three.
func fileDSN(path, params string) string {
	escaped := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(filepath.ToSlash(path))

	return "file:" + escaped + "?" + params
}

// openDatabase is the database file at path for writing, with the
// shared-access pragmas, made where there is none.
func openDatabase(path string) *sql.DB {
	return connect(fileDSN(path, fmt.Sprintf(dsnPragmas, busyTimeoutMillis)))
}

// openAsItIs is the database named, already on disk, for reading alone: it
// makes no directory, prepares no schema, narrows no mode and writes no row.
// Like any reader of a write-ahead-logged database, SQLite may leave the log's
// two companion files beside it, owner-only, until the next live open clears
// them.
func (s Store) openAsItIs(name string) *sql.DB {
	return connect(fileDSN(filepath.Join(s.dir, name), fmt.Sprintf(readOnlyPragmas, busyTimeoutMillis)))
}
