// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package store keeps a little workflow state on disk between sessions, in two
// SQLite databases under the OS-native data directory. workflow.db is the
// cache, what a session can see again: the commit scope last used in a
// repository, what was announced, and the last issue list seen. kept.db is
// what the user decided and a session cannot see again: whom a forge owner is
// on Slack, a repository's groups, and the favorite directories. Neither ever
// holds a secret, and callers key them only by credential-free identifiers.
//
// Each schema is Third Normal Form and every table is STRICT: repeating
// groups — the cached issue list — are a parent row and one child row each, no
// derived value is stored, and each column's type is enforced at write time.
// Timestamps are RFC3339 UTC text, a child's foreign key cascades from the row
// that owns it and restricts toward a Slack user or group others share, and a
// write that replaces a group runs in one transaction. Callers sanitize text
// on the way in and treat what they read back as untrusted, since a file on
// disk can be tampered.
//
// A store is a value, and one that is off no-ops every method, so a caller need
// not special-case the privacy opt-out. Each operation opens its own short-lived
// connection, so there is no handle to close and the terminal interface and the
// web server can share the files; WAL and a busy timeout keep their writes from
// colliding.
//
// Each file's schema has one version, stamped into it, and a file written at
// another is discarded and started fresh: what workflow.db held is seen
// again, and what kept.db held is asked for again. Neither has migrations.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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

// schemaVersion is the version of the schema cacheTables makes, stamped into
// the file as SQLite's user_version. Bump it whenever a CREATE TABLE changes: a
// file at another version is discarded whole and made again, so there are no
// migrations to write.
const schemaVersion = 4

// ErrNoDir reports that no OS-native data directory could be determined.
var ErrNoDir = errors.New("could not determine a data directory for the store")

// timestampLayout is RFC3339 with every nanosecond digit written, so two
// moments within one second differ and the text sorts as the moments do.
const timestampLayout = "2006-01-02T15:04:05.000000000Z07:00"

// timestamp formats a moment as the RFC3339 UTC string the store's _at columns
// hold, so a STRICT table keeps them as TEXT rather than a forbidden DATETIME.
// Its fixed width is what lets SQL compare two of them as text.
func timestamp(now time.Time) string {
	return now.UTC().Format(timestampLayout)
}

// LastScope is the commit scope last recorded for repo, and whether one was. A
// disabled store, or a repo with nothing recorded, reports no scope.
func (s Store) LastScope(ctx context.Context, repo string) (string, bool, error) {
	if repo == "" {
		return "", false, nil
	}

	database, found, err := s.readFile(ctx, cacheFile())
	if err != nil || !found {
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

	database, err := s.openFile(ctx, cacheFile())
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

// makeDir makes the store directory, readable only by its owner.
func (s Store) makeDir() error {
	err := os.MkdirAll(s.dir, dirPerm)
	if err != nil {
		return fmt.Errorf("creating the store directory: %w", err)
	}

	// MkdirAll leaves a directory that already exists at its own mode, so narrow
	// it here rather than only on the path that created it.
	restrictToOwner(s.dir, dirPerm)

	return nil
}

// openCurrent opens the database at path for writing, at this build's schema
// version: a file another build's schema left there is discarded first, and a
// fresh or remade file is stamped before it holds a table, so a second process
// opening it meanwhile meets this version, never an older build's file to
// discard. Two processes that both open an old file before either discards it
// each discard the other's remade file; the file on disk ends consistent, and
// only what the loser wrote that session is lost.
func openCurrent(ctx context.Context, path string, file storeFile) (*sql.DB, error) {
	database := openDatabase(path, file.pragmas)

	state, err := readStateOnOpen(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	if state.version == file.version {
		return database, nil
	}

	if state.holdsTables {
		_ = database.Close()

		database, err = remakeDatabase(path, file.pragmas)
		if err != nil {
			return nil, err
		}
	}

	err = stamp(ctx, database, file.version)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	return database, nil
}

// rowReader reads one row: a database, or a transaction in one.
type rowReader interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// fileState is what a database file holds as it is read: the schema version
// stamped in it, and whether it holds a table, which a fresh file does not,
// so one at no version is new rather than another build's.
type fileState struct {
	version     int
	holdsTables bool
}

// readState reads the version stamped in the file and whether it holds a
// table, in one statement. A file that is not a database, or whose schema
// cannot be read, fails here, since the connection's own pragmas load the
// schema; either is reported and left alone.
func readState(ctx context.Context, database rowReader) (fileState, error) {
	var state fileState

	err := database.QueryRowContext(ctx,
		`SELECT user_version, EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table') FROM pragma_user_version`).
		Scan(&state.version, &state.holdsTables)
	if err != nil {
		return fileState{}, fmt.Errorf("reading the store schema version: %w", err)
	}

	return state, nil
}

// openingRetries and openingPause bound how long a connection's first read
// waits out another connection switching a fresh file into WAL. Both switches
// hold the file's shared lock and want its exclusive one, so SQLite fails one
// with SQLITE_BUSY at once rather than through the busy timeout, which would
// deadlock them.
const (
	openingRetries = 50
	openingPause   = 20 * time.Millisecond
)

// primaryCode masks an extended SQLite result code to its primary one.
const primaryCode = 0xff

// readStateOnOpen is readState on a connection's first use, which runs the
// connection's pragmas, journal_mode(WAL) among them: it reads again while
// that switch loses to another connection's, since by then the file is in WAL
// and the switch costs nothing.
func readStateOnOpen(ctx context.Context, database *sql.DB) (fileState, error) {
	state, err := readState(ctx, database)

	for attempt := 0; attempt < openingRetries && isBusy(err); attempt++ {
		time.Sleep(openingPause)

		state, err = readState(ctx, database)
	}

	return state, err
}

// isBusy reports SQLite failing for a lock another connection holds.
func isBusy(err error) bool {
	var failure *sqlite.Error

	return errors.As(err, &failure) && failure.Code()&primaryCode == sqlite3.SQLITE_BUSY
}

// inTransaction runs write in one transaction on database, committing only
// when write succeeds, so a write that fails part way leaves nothing of what
// it did. A failure to begin or commit is reported as doing, what the
// transaction was for.
func inTransaction(ctx context.Context, database *sql.DB, doing string, write func(*sql.Tx) error) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}
	defer func() { _ = transaction.Rollback() }()

	err = write(transaction)
	if err != nil {
		return err
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}

	return nil
}

// remakeDatabase discards the database at path and opens a fresh one there
// with pragmas.
func remakeDatabase(path, pragmas string) (*sql.DB, error) {
	err := removeDatabase(path)
	if err != nil {
		return nil, err
	}

	return openDatabase(path, pragmas), nil
}

// stamp writes version, this build's schema version for the file, into it. A
// PRAGMA binds no placeholder, so the one statement built from a string holds
// only a version this package's own constants name.
func stamp(ctx context.Context, database *sql.DB, version int) error {
	// The open that just made or read this file holds it, so the stamp fails
	// only when the file changes between the two calls.
	_, err := database.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(version))
	if err != nil {
		return fmt.Errorf("stamping the store schema version: %w", err)
	}

	return nil
}

// removeDatabase deletes the database at path along with the -wal and -shm
// files SQLite may have left beside it; one already gone is no error.
func removeDatabase(path string) error {
	for _, each := range companionsOf(path) {
		// The open that just read the version held these files, so a removal
		// fails only when one changes underneath between the two calls.
		err := os.Remove(each)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("discarding the store at another schema version: %w", err)
		}
	}

	return nil
}

// restrictToOwner sets path to perm. A failed chmod is tolerated: a filesystem
// without Unix modes (vfat, SMB) cannot keep them, and the store holds no
// secret, so it keeps working there rather than switching itself off.
func restrictToOwner(path string, perm os.FileMode) {
	_ = os.Chmod(path, perm)
}

// cacheFile is workflow.db, the cache.
func cacheFile() storeFile {
	return storeFile{name: dbName, pragmas: dsnPragmas, version: schemaVersion, tables: cacheTables}
}

// cacheTables makes every table the cache holds where it is missing.
func cacheTables() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS scopes (
			repo       TEXT NOT NULL PRIMARY KEY,
			scope      TEXT NOT NULL,
			updated_at TEXT NOT NULL
		) STRICT`,
		// An announcement is claimed before it is posted, so one key is posted
		// once whoever asks: claim is the generation, one at first and one more
		// for each post asked for again or claim taken over one given up, so
		// it never repeats for a key while the file lives; first_claim is the
		// generation the claim held now began at, the only one that may be
		// given up; released is 1 once it was, its post surely not gone; and
		// posted_at is NULL until the post is confirmed.
		`CREATE TABLE IF NOT EXISTS announces (
			repo       TEXT NOT NULL,
			pull       INTEGER NOT NULL,
			moment     INTEGER NOT NULL,
			claim       INTEGER NOT NULL,
			first_claim INTEGER NOT NULL,
			claimed_at  TEXT NOT NULL,
			posted_at   TEXT,
			released    INTEGER NOT NULL DEFAULT 0 CHECK (released IN (0, 1)),
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
}
