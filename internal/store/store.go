// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package store keeps a little workflow state on disk between sessions — the
// commit scope last used in a repository today, and what was announced and the
// last issue list seen as later features land — in a SQLite database under the
// OS-native data directory. It never holds a secret.
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
// connection, which is what lets two processes share the one file.
const dsnPragmas = "?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)"

// ErrNoDir reports that no OS-native data directory could be determined.
var ErrNoDir = errors.New("could not determine a data directory for the store")

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

	_, err = database.ExecContext(ctx,
		`INSERT INTO scopes (repo, scope, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(repo) DO UPDATE SET scope = excluded.scope, updated_at = excluded.updated_at`,
		repo, scope, now.Unix(),
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
	_, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS scopes (
		repo       TEXT PRIMARY KEY,
		scope      TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("preparing the store schema: %w", err)
	}

	_, err = database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS announces (
		repo         TEXT NOT NULL,
		pull         INTEGER NOT NULL,
		moment       INTEGER NOT NULL,
		announced_at INTEGER NOT NULL,
		PRIMARY KEY (repo, pull, moment)
	)`)
	if err != nil {
		return fmt.Errorf("preparing the store schema: %w", err)
	}

	return nil
}
