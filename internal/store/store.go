// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package store keeps a little workflow state on disk between sessions — the
// commit scope last used in a repository, what was announced, and the last issue
// list seen — in a SQLite database under the OS-native data directory. It never
// holds a secret, and its callers key it only by credential-free identifiers.
//
// The schema is Third Normal Form and every table is STRICT: repeating groups —
// the cached issue list — are a parent row and one child row each, no derived
// value is stored, and each column's type is enforced at write time. Timestamps
// are RFC3339 UTC text, foreign keys cascade, and a write that replaces a group
// runs in one transaction. Callers sanitize text on the way in and treat what
// they read back as untrusted, since a file on disk can be tampered.
//
// A store is a value, and a disabled one no-ops every method, so a caller need
// not special-case the privacy opt-out. Each operation opens its own short-lived
// connection, so there is no handle to close and the terminal interface and the
// web server can share the file; WAL and a busy timeout keep their writes from
// colliding.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver, so CGO stays off
)

// The store is readable only by its owner, where the filesystem keeps Unix
// modes: the directory is not traversable and the database not readable by
// anyone else, which also guards the -wal and -shm files SQLite writes beside
// it.
const (
	dirPerm  os.FileMode = 0o700
	filePerm os.FileMode = 0o600
)

// dbName is the database file's name inside the store directory.
const dbName = "workflow.db"

// busyTimeoutMillis is how long a write waits for another connection's lock
// before giving up, so the interface and the web server sharing the file do not
// fail on momentary contention.
const busyTimeoutMillis = 5000

// dsnPragmas turns on write-ahead logging and the busy timeout for every
// connection, which is what lets two processes share the one file, and foreign
// keys, which SQLite enforces per-connection so an ON DELETE CASCADE only fires
// when it is on.
const dsnPragmas = "?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"

// readOnlyDSN opens the database file named in it for reading only, never
// creating it, with the same busy timeout, so a read waits out another
// connection's lock rather than failing.
const readOnlyDSN = "file:%s?mode=ro&_pragma=busy_timeout(%d)"

// ErrNoDir reports that no OS-native data directory could be determined.
var ErrNoDir = errors.New("could not determine a data directory for the store")

// timestamp formats a moment as the RFC3339 UTC string the store's _at columns
// hold, so a STRICT table keeps them as TEXT rather than a forbidden DATETIME.
func timestamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

// Store points at the on-disk store. Its zero value, and any disabled store,
// no-op every method, so persistence-off behaves exactly as no store at all.
type Store struct {
	dir      string
	disabled bool
	readOnly bool
}

// New points a store at dir. Disabled keeps the code path identical for a user
// who opts out of persistence: every method no-ops.
func New(dir string, disabled bool) Store {
	return Store{dir: dir, disabled: disabled}
}

// ReadOnly is the store for a dry run: it reads what is already on disk, reads
// nothing where there is no store yet rather than create one, and no-ops every
// write.
func (s Store) ReadOnly() Store {
	return Store{dir: s.dir, disabled: s.disabled, readOnly: true}
}

// LastScope is the commit scope last recorded for repo, and whether one was. A
// disabled store, or a repo with nothing recorded, reports no scope.
func (s Store) LastScope(ctx context.Context, repo string) (string, bool, error) {
	if s.nothingToRead() || repo == "" {
		return "", false, nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = database.Close() }()

	var scope string

	err = database.QueryRowContext(ctx, `SELECT scope FROM scopes WHERE repo = ?`, repo).Scan(&scope)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("reading the last scope: %w", err)
	default:
		return scope, true, nil
	}
}

// RecordScope remembers scope as the one last used in repo, replacing any earlier
// one. A disabled or read-only store records nothing.
func (s Store) RecordScope(ctx context.Context, repo, scope string, now time.Time) error {
	if s.writesNothing() || repo == "" {
		return nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	_, err = database.ExecContext(
		ctx,
		`INSERT INTO scopes (repo, scope, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(repo) DO UPDATE SET scope = excluded.scope, updated_at = excluded.updated_at`,
		repo, scope, timestamp(now),
	)
	if err != nil {
		return fmt.Errorf("recording the scope: %w", err)
	}

	return nil
}

// off reports a store that should do nothing: disabled, or with nowhere to write.
func (s Store) off() bool {
	return s.disabled || s.dir == ""
}

// writesNothing reports a store whose writes no-op: one that is off, or read-only.
func (s Store) writesNothing() bool {
	return s.off() || s.readOnly
}

// nothingToRead reports a store with nothing it may read: one that is off, or a
// read-only one with no database on disk, which it must not create by opening.
func (s Store) nothingToRead() bool {
	switch {
	case s.off():
		return true
	case !s.readOnly:
		return false
	default:
		_, err := os.Stat(filepath.Join(s.dir, dbName))

		return err != nil
	}
}

// open makes the store directory, opens the database with the shared-access
// pragmas, prepares the schema, and restricts the directory and file to their
// owner. A read-only store opens the database as it is instead.
func (s Store) open(ctx context.Context) (*sql.DB, error) {
	if s.readOnly {
		return s.openAsItIs()
	}

	err := os.MkdirAll(s.dir, dirPerm)
	if err != nil {
		return nil, fmt.Errorf("creating the store directory: %w", err)
	}

	// MkdirAll leaves a directory that already exists at its own mode, so narrow
	// it here rather than only on the path that created it.
	restrictToOwner(s.dir, dirPerm)

	path := filepath.Join(s.dir, dbName)

	database, err := sql.Open("sqlite", path+fmt.Sprintf(dsnPragmas, busyTimeoutMillis))
	if err != nil {
		return nil, fmt.Errorf("opening the store: %w", err)
	}

	err = migrate(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	// The schema step created the file honoring the umask; narrow it now that it
	// exists, so the database is readable only by its owner.
	restrictToOwner(path, filePerm)

	return database, nil
}

// openAsItIs opens the database already on disk for reading alone: it makes no
// directory, prepares no schema, narrows no mode and writes no row. Like any
// reader of a write-ahead-logged database, SQLite may leave the log's two
// companion files beside it, owner-only, until the next live open clears them.
func (s Store) openAsItIs() (*sql.DB, error) {
	// Read-only takes a URI, where a percent sign escapes, a question mark starts
	// the parameters and a hash ends the path, so the file's name escapes them.
	name := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").
		Replace(filepath.ToSlash(filepath.Join(s.dir, dbName)))

	database, err := sql.Open("sqlite", fmt.Sprintf(readOnlyDSN, name, busyTimeoutMillis))
	if err != nil {
		return nil, fmt.Errorf("opening the store: %w", err)
	}

	return database, nil
}

// restrictToOwner sets path to perm. A failed chmod is tolerated: a filesystem
// without Unix modes (vfat, SMB) cannot keep them, and the store holds no
// secret, so it keeps working there rather than switching itself off.
func restrictToOwner(path string, perm os.FileMode) {
	_ = os.Chmod(path, perm)
}

// migrate brings the schema up to date. It is forward-only and idempotent, so
// every open can run it.
func migrate(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS scopes (
			repo       TEXT NOT NULL PRIMARY KEY,
			scope      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS announces (
			repo         TEXT NOT NULL,
			pull         INTEGER NOT NULL,
			moment       INTEGER NOT NULL,
			announced_at TEXT NOT NULL,
			PRIMARY KEY (repo, pull, moment)
		) STRICT`,
		// The cache is a parent row per view — which records that a view was cached
		// even when it returned no issues — and a child row per issue, so no column
		// holds a repeating group.
		`CREATE TABLE IF NOT EXISTS issue_cache (
			instance  TEXT NOT NULL,
			view      TEXT NOT NULL,
			cached_at TEXT NOT NULL,
			PRIMARY KEY (instance, view)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS cached_issue (
			instance        TEXT NOT NULL,
			view            TEXT NOT NULL,
			position        INTEGER NOT NULL,
			issue_key       TEXT NOT NULL,
			summary         TEXT NOT NULL,
			status          TEXT NOT NULL,
			status_category TEXT NOT NULL,
			type            TEXT NOT NULL,
			priority        TEXT NOT NULL,
			PRIMARY KEY (instance, view, position),
			FOREIGN KEY (instance, view) REFERENCES issue_cache(instance, view) ON DELETE CASCADE
		) STRICT`,
	}

	for _, statement := range statements {
		_, err := database.ExecContext(ctx, statement)
		if err != nil {
			return fmt.Errorf("preparing the store schema: %w", err)
		}
	}

	return nil
}
