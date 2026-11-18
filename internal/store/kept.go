// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
)

// keptName is the kept database's file name, beside the cache's dbName. It
// holds what the user decided — whom a forge owner is on Slack, which groups
// a repository tags, the favorite directories — which a session cannot see
// again; it is opened as the cache is, so a file at another schema version is
// discarded and made again, and what it held is answered again.
const keptName = "kept.db"

// keptPragmas are the cache's shared-access pragmas, with every transaction
// taking the write lock as it begins. A kept write reads before it writes in
// one transaction, and in WAL a deferred transaction whose snapshot another
// connection's commit has moved past fails its write with SQLITE_BUSY at
// once, without the busy timeout; taking the lock first makes it wait the
// timeout out instead.
const keptPragmas = dsnPragmas + "&_txlock=immediate"

// keptSchemaVersion is the version of the schema keptSchema makes, stamped
// into the file as SQLite's user_version. Bump it whenever a kept table
// changes: a file at another version is discarded whole and made again, so
// there are no migrations to write, and the owner links, groups and
// favorites it held are forgotten.
const keptSchemaVersion = 2

// keptFile is kept.db, what the user decided.
func keptFile() storeFile {
	return storeFile{name: keptName, pragmas: keptPragmas, version: keptSchemaVersion, tables: keptSchema}
}

// keptSchema is every table the kept database holds.
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

	database, err := s.openFile(ctx, keptFile())
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	return inTransaction(ctx, database, "writing the kept data", write)
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
