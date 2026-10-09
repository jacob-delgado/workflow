// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/jacob-delgado/workflow/internal/sqlitefile"
)

// keptName is the kept database's file name, beside the cache's dbName. It holds
// what the user decided — whom a forge owner is on Slack, which groups a
// repository tags — which a session cannot see again, so unlike the cache it is
// never discarded.
const keptName = "kept.db"

// keptPragmas are the cache's shared-access pragmas, with every transaction
// taking the write lock as it begins: making the schema re-reads the version
// under that lock, so two processes opening a fresh file never both make it.
const keptPragmas = dsnPragmas + "&_txlock=immediate"

// keptSchemaVersion is the version of the schema keptSchema makes, stamped
// into the file as SQLite's user_version. There are no migrations: a change to
// a kept table changes keptSchema and bumps this, and a file at another
// version is left for the user to remove.
//
// Trade-off TRADE-25: kept data has one schema and is never migrated.
const keptSchemaVersion = 2

// ErrKeptSchemaDiffers reports a kept database written at another schema
// version than this build's. It is left as it is: it reads as empty, and a
// write is refused rather than risk what it holds.
var ErrKeptSchemaDiffers = errors.New(
	"the kept data was written by a build of workflow with another schema; " +
		"run `workflow db-clean --all` to start it fresh")

// keptSchema is every table the kept database holds, made once in a fresh
// file.
func keptSchema() []string {
	return slices.Concat(ownersSchema(), groupsSchema(), favoritesSchema())
}

// keptWithin runs write in one transaction on the kept database, at this
// build's schema, committing only when write succeeds. A disabled or read-only
// store writes nothing.
func (s Store) keptWithin(ctx context.Context, write func(*sql.Tx) error) error {
	if s.writesNothing() {
		return nil
	}

	database, err := s.openKept(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	return inTransaction(ctx, database, "writing the kept data", write)
}

// readKept opens the kept database for a read, and reports false when there
// is nothing to read: a store that is off, a read-only one with no file or one
// not yet made, or a file at another schema version.
func (s Store) readKept(ctx context.Context) (*sql.DB, bool, error) {
	if s.nothingToReadIn(keptName) {
		return nil, false, nil
	}

	if s.readOnly {
		return s.readKeptAsItIs(ctx)
	}

	database, err := s.openKept(ctx)

	switch {
	case errors.Is(err, ErrKeptSchemaDiffers):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	default:
		return database, true, nil
	}
}

// readKeptAsItIs opens the kept database already on disk for a dry run's
// read, which makes nothing, so a file not at this build's schema reads as
// empty.
func (s Store) readKeptAsItIs(ctx context.Context) (*sql.DB, bool, error) {
	database := s.openAsItIs(keptName)

	state, err := readState(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, false, err
	}

	if state.version != keptSchemaVersion {
		_ = database.Close()

		return nil, false, nil
	}

	return database, true, nil
}

// openKept opens the kept database for writing, making it and its schema
// where there is none, and refuses a file at another schema version.
func (s Store) openKept(ctx context.Context) (*sql.DB, error) {
	err := s.makeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, keptName)
	database := sqlitefile.Open(path, fmt.Sprintf(keptPragmas, busyTimeoutMillis))

	err = prepareKept(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	restrictToOwner(path, filePerm)

	return database, nil
}

// prepareKept makes the schema in a fresh kept database and refuses one at
// another version. A file already at this build's costs one read and no lock.
func prepareKept(ctx context.Context, database *sql.DB) error {
	return settleKept(
		func() (fileState, error) { return readStateOnOpen(ctx, database) },
		func() error { return makeKeptSchema(ctx, database) },
	)
}

// makeKeptSchema makes the schema in one transaction, which takes the write
// lock as it begins and settles the version again under it, since another
// process may have made it meanwhile.
func makeKeptSchema(ctx context.Context, database *sql.DB) error {
	return inTransaction(ctx, database, "preparing the kept data", func(transaction *sql.Tx) error {
		return settleKept(
			func() (fileState, error) { return readState(ctx, transaction) },
			func() error { return writeKeptSchema(ctx, transaction) },
		)
	})
}

// settleKept reads the kept file's version and acts on it: a file at this
// build's version is ready, one at none has its schema made, and one at
// another is refused.
func settleKept(read func() (fileState, error), makeSchema func() error) error {
	state, err := read()
	if err != nil {
		return err
	}

	switch state.version {
	case keptSchemaVersion:
		return nil
	case 0:
		return makeSchema()
	default:
		return ErrKeptSchemaDiffers
	}
}

// writeKeptSchema makes every kept table and stamps this build's version. A
// PRAGMA binds no placeholder; the version is this package's own constant.
func writeKeptSchema(ctx context.Context, transaction *sql.Tx) error {
	stampVersion := "PRAGMA user_version = " + strconv.Itoa(keptSchemaVersion)

	for _, statement := range append(keptSchema(), stampVersion) {
		_, err := transaction.ExecContext(ctx, statement)
		if err != nil {
			return fmt.Errorf("preparing the kept data: %w", err)
		}
	}

	return nil
}

// pruneSlackEntities drops every Slack user or group nothing links to any
// more, so the file keeps no one it has no use for.
func pruneSlackEntities(ctx context.Context, transaction *sql.Tx) error {
	_, err := transaction.ExecContext(ctx,
		`DELETE FROM slack_entity
			WHERE slack_id NOT IN (SELECT slack_id FROM owner_slack)
			AND slack_id NOT IN (SELECT slack_id FROM repo_group)`)
	if err != nil {
		return fmt.Errorf("pruning the Slack entities: %w", err)
	}

	return nil
}
