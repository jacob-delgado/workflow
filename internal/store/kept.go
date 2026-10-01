// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
)

// keptName is the kept database's file name, beside the cache's dbName. It holds
// what the user decided — whom a forge owner is on Slack, which groups a
// repository tags — which a session cannot see again, so unlike the cache it is
// never discarded.
const keptName = "kept.db"

// keptPragmas are the cache's shared-access pragmas, with every transaction
// taking the write lock as it begins: a migration re-reads the version under
// that lock, so two processes opening an older file never both apply one.
const keptPragmas = dsnPragmas + "&_txlock=immediate"

// ErrKeptFromNewerBuild reports a kept database a newer build of workflow
// migrated past what this build knows. It is left as it is: it reads as empty,
// and a write is refused rather than risk what the newer build kept.
var ErrKeptFromNewerBuild = errors.New("the kept data is from a newer build of workflow; it is left as it is")

// keptMigrations are the kept database's migrations in order; the file's
// user_version is how many of them it has applied. Add a migration at the end,
// never edit or remove one: a file in the field may already hold it.
//
// Trade-off TRADE-25: kept data migrates forward and is never discarded, so
// this list only grows, where the cache's schema is simply remade.
func keptMigrations() [][]string {
	return [][]string{
		ownersMigration(), groupsMigration(), workspacesMigration(), ownerKindsMigration(), linkWorkspacesMigration(),
	}
}

// keptWithin runs write in one transaction on the kept database, migrated to
// this build, committing only when write succeeds. A disabled or read-only
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

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("writing the kept data: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	err = write(transaction)
	if err != nil {
		return err
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("writing the kept data: %w", err)
	}

	return nil
}

// readKept opens the kept database for a read that needs the tables the first
// need migrations make, and reports false when there is nothing to read: a
// store that is off, a read-only one with no file or a file not yet migrated
// that far, or a file from a newer build.
func (s Store) readKept(ctx context.Context, need int) (*sql.DB, bool, error) {
	if s.nothingToReadIn(keptName) {
		return nil, false, nil
	}

	if s.readOnly {
		return s.readKeptAsItIs(ctx, need)
	}

	database, err := s.openKept(ctx)

	switch {
	case errors.Is(err, ErrKeptFromNewerBuild):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	default:
		return database, true, nil
	}
}

// readKeptAsItIs opens the kept database already on disk for a dry run's
// read, which migrates nothing, so a file short of need migrations, or past
// this build's, reads as empty.
func (s Store) readKeptAsItIs(ctx context.Context, need int) (*sql.DB, bool, error) {
	database, err := s.openAsItIs(keptName)
	if err != nil {
		return nil, false, err
	}

	version, err := readVersion(ctx, database)
	if err != nil {
		_ = database.Close()

		return nil, false, err
	}

	if version < need || version > len(keptMigrations()) {
		_ = database.Close()

		return nil, false, nil
	}

	return database, true, nil
}

// openKept opens the kept database for writing, making it where there is none,
// and migrates it to this build.
func (s Store) openKept(ctx context.Context) (*sql.DB, error) {
	err := s.makeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(s.dir, keptName)

	// Trade-off TRADE-15: sql.Open fails only for a driver not registered, and
	// this package imports its driver.
	database, err := sql.Open("sqlite", path+fmt.Sprintf(keptPragmas, busyTimeoutMillis))
	if err != nil {
		return nil, fmt.Errorf("opening the kept data: %w", err)
	}

	err = migrate(ctx, database, keptMigrations())
	if err != nil {
		_ = database.Close()

		return nil, err
	}

	restrictToOwner(path, filePerm)

	return database, nil
}

// migrate brings the kept database up to every migration, refusing a file
// past them. A file already current costs one read and no lock.
func migrate(ctx context.Context, database *sql.DB, migrations [][]string) error {
	version, err := readVersion(ctx, database)
	if err != nil {
		return err
	}

	switch {
	case version == len(migrations):
		return nil
	case version > len(migrations):
		return ErrKeptFromNewerBuild
	default:
		return applyMigrations(ctx, database, migrations)
	}
}

// applyMigrations applies the migrations the file lacks in one transaction,
// which takes the write lock as it begins and re-reads the version under it,
// since another process may have migrated the file meanwhile.
func applyMigrations(ctx context.Context, database *sql.DB, migrations [][]string) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrating the kept data: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	version, err := readVersion(ctx, transaction)
	if err != nil {
		return err
	}

	if version > len(migrations) {
		return ErrKeptFromNewerBuild
	}

	for _, migration := range migrations[version:] {
		for _, statement := range migration {
			_, err = transaction.ExecContext(ctx, statement)
			if err != nil {
				return fmt.Errorf("migrating the kept data: %w", err)
			}
		}
	}

	// A PRAGMA binds no placeholder; the version is this package's own count.
	_, err = transaction.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(len(migrations)))
	if err != nil {
		return fmt.Errorf("stamping the kept data's version: %w", err)
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("migrating the kept data: %w", err)
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
