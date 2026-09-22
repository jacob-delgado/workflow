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
	"time"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver, so CGO stays off
)

// The store is readable only by its owner: the directory is not traversable and
// the database not readable by anyone else, which also guards the -wal and -shm
// files SQLite writes beside it.
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
}

// New points a store at dir. Disabled keeps the code path identical for a user
// who opts out of persistence: every method no-ops.
func New(dir string, disabled bool) Store {
	return Store{dir: dir, disabled: disabled}
}

// LastScope is the commit scope last recorded for repo, and whether one was. A
// disabled store, or a repo with nothing recorded, reports no scope.
func (s Store) LastScope(ctx context.Context, repo string) (string, bool, error) {
	if s.off() || repo == "" {
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
// one. A disabled store records nothing.
func (s Store) RecordScope(ctx context.Context, repo, scope string, now time.Time) error {
	if s.off() || repo == "" {
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

// open makes the store directory, opens the database with the shared-access
// pragmas, prepares the schema, and restricts the file to its owner.
func (s Store) open(ctx context.Context) (*sql.DB, error) {
	err := os.MkdirAll(s.dir, dirPerm)
	if err != nil {
		return nil, fmt.Errorf("creating the store directory: %w", err)
	}

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
	_ = os.Chmod(path, filePerm)

	return database, nil
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
