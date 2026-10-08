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
// Timestamps are RFC3339 UTC text, foreign keys cascade, and a write that
// replaces a group runs in one transaction. Callers sanitize text on the way
// in and treat what they read back as untrusted, since a file on disk can be
// tampered.
//
// A store is a value, and a disabled one no-ops every method, so a caller need
// not special-case the privacy opt-out. Each operation opens its own short-lived
// connection, so there is no handle to close and the terminal interface and the
// web server can share the files; WAL and a busy timeout keep their writes from
// colliding.
//
// Each file's schema has one version, stamped into it. A workflow.db written
// at another is discarded and started fresh, since what it held is seen
// again. A kept.db written at another is never discarded: it is left as it
// is, for the user to remove.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// schemaVersion is the version of the schema createSchema makes, stamped into
// the file as SQLite's user_version. Bump it whenever a CREATE TABLE changes: a
// file at another version is discarded whole and made again, so there are no
// migrations to write.
const schemaVersion = 1

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

// Keeps reports a store that keeps anything: neither disabled nor without a
// directory. A read-only store keeps what is already on disk, so it keeps.
func (s Store) Keeps() bool {
	return !s.off()
}

// Writes reports a store that records what it is told: kept on, somewhere to
// keep it, and not read-only.
func (s Store) Writes() bool {
	return !s.writesNothing()
}

// off reports a store that should do nothing: disabled, or with nowhere to write.
func (s Store) off() bool {
	return s.disabled || s.dir == ""
}

// writesNothing reports a store whose writes no-op: one that is off, or read-only.
func (s Store) writesNothing() bool {
	return s.off() || s.readOnly
}

// nothingToRead reports a store with nothing it may read from the cache.
func (s Store) nothingToRead() bool {
	return s.nothingToReadIn(dbName)
}

// nothingToReadIn reports a store with nothing it may read from the database
// named: one that is off, or a read-only one with no such file on disk, which
// it must not create by opening.
func (s Store) nothingToReadIn(name string) bool {
	switch {
	case s.off():
		return true
	case !s.readOnly:
		return false
	default:
		_, err := os.Stat(filepath.Join(s.dir, name))

		return err != nil
	}
}

// open makes the store directory, opens the database at this build's schema
// version — discarding one another build left — creates the tables it is
// missing, and restricts the directory and file to their owner. A read-only
// store opens the database as it is instead.
func (s Store) open(ctx context.Context) (*sql.DB, error) {
	if s.readOnly {
		return s.openAsItIs(dbName), nil
	}

	err := s.makeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, dbName)

	database, err := openCurrent(ctx, path)
	if err != nil {
		return nil, err
	}

	err = createSchema(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	// The open created the file honoring the umask; narrow it now that it
	// exists, so the database is readable only by its owner.
	restrictToOwner(path, filePerm)

	return database, nil
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
func openCurrent(ctx context.Context, path string) (*sql.DB, error) {
	database := openDatabase(path)

	state, err := readStateOnOpen(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	if state.version == schemaVersion {
		return database, nil
	}

	if state.holdsTables {
		_ = database.Close()

		database, err = remakeDatabase(path)
		if err != nil {
			return nil, err
		}
	}

	err = stamp(ctx, database)
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

// remakeDatabase discards the database at path and opens a fresh one there.
func remakeDatabase(path string) (*sql.DB, error) {
	err := removeDatabase(path)
	if err != nil {
		return nil, err
	}

	return openDatabase(path), nil
}

// stamp writes this build's schema version into the file. A PRAGMA binds no
// placeholder, so the one statement built from a string holds only this
// package's own constant.
func stamp(ctx context.Context, database *sql.DB) error {
	// Trade-off TRADE-16: the open that just made or read this file holds it,
	// so the stamp fails only when the file changes between the two calls.
	_, err := database.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(schemaVersion))
	if err != nil {
		return fmt.Errorf("stamping the store schema version: %w", err)
	}

	return nil
}

// removeDatabase deletes the database at path along with the -wal and -shm
// files SQLite may have left beside it; one already gone is no error.
func removeDatabase(path string) error {
	for _, each := range companionsOf(path) {
		// Trade-off TRADE-16: the open that just read the version held these
		// files, so a removal fails only when one changes underneath between
		// the two calls.
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

// createSchema makes every table the store needs where it is missing. It never
// alters a table: a file at another version was discarded before this ran, so
// what it meets is this version's schema or nothing.
func createSchema(ctx context.Context, database *sql.DB) error {
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
