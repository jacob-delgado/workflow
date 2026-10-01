// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DataKind says what a database file holds: conveniences a session sees again,
// or what the user decided.
type DataKind string

// The kinds of database file the store keeps.
const (
	DataCache DataKind = "cache"
	DataKept  DataKind = "kept"
)

// CleanScope says how much a clean removes.
type CleanScope int

// The scopes of a clean: the cache alone, or the kept data too.
const (
	CleanCache CleanScope = iota
	CleanAll
)

// asideSuffix marks a file a clean has set aside and is about to remove.
const asideSuffix = ".cleaning"

// ErrCleanRefused reports a clean that would reach past the store's own plain
// files: a relative directory, or a symlink or directory where a database file
// belongs. Nothing is removed.
var ErrCleanRefused = errors.New("not the store's own files to clean")

// ErrNotCleaned reports a file a clean could not remove, as one another
// program holds open can be on Windows.
var ErrNotCleaned = errors.New("could not remove a store file")

// Held is a count of one kind of thing a database file holds.
type Held struct {
	What  string
	Count int
}

// DataFile is one database file in the store directory: its name, its kind,
// its size with its write-ahead log and shared-memory companions, and what it
// holds, where that could be read.
type DataFile struct {
	Name  string
	Kind  DataKind
	Bytes int64
	Holds []Held
}

// heldTable is one table a summary counts: its name, what its rows are, and
// the query that counts them, written out whole so no SQL is built.
type heldTable struct {
	table string
	what  string
	count string
}

// database is one database file the store keeps, and the tables a summary
// counts in it.
type database struct {
	name   string
	kind   DataKind
	tables []heldTable
}

// databases are the store's database files, the cache first.
func databases() []database {
	return []database{
		{name: dbName, kind: DataCache, tables: []heldTable{
			{"scopes", "scopes", `SELECT COUNT(*) FROM scopes`},
			{"announces", "announcements", `SELECT COUNT(*) FROM announces`},
			{"issue_cache", "cached views", `SELECT COUNT(*) FROM issue_cache`},
			{"cached_issue", "cached issues", `SELECT COUNT(*) FROM cached_issue`},
		}},
		{name: keptName, kind: DataKept, tables: []heldTable{
			{"owner_decision", "owner decisions", `SELECT COUNT(*) FROM owner_decision`},
			{"owner_slack", "owners on Slack", `SELECT COUNT(*) FROM owner_slack`},
			{"slack_entity", "Slack users and groups", `SELECT COUNT(*) FROM slack_entity`},
			{"repo_group", "repository groups", `SELECT COUNT(*) FROM repo_group`},
			{"repo_choice_group", "chosen groups", `SELECT COUNT(*) FROM repo_choice_group`},
		}},
	}
}

// Files lists the database files in dir, each with its size and what it holds,
// read without writing; a file not there is left out.
func Files(ctx context.Context, dir string) ([]DataFile, error) {
	var files []DataFile

	for _, each := range databases() {
		size, found, err := sizeOf(filepath.Join(dir, each.name))
		if err != nil {
			return nil, err
		}

		if found {
			files = append(files, DataFile{
				Name: each.name, Kind: each.kind, Bytes: size, Holds: summarize(ctx, dir, each),
			})
		}
	}

	return files, nil
}

// sizeOf is the size of the database at path with its companions, and whether
// the database is there.
func sizeOf(path string) (int64, bool, error) {
	var size int64

	for index, each := range companionsOf(path) {
		info, err := os.Lstat(each)

		switch {
		case errors.Is(err, fs.ErrNotExist) && index == 0:
			return 0, false, nil
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return 0, false, fmt.Errorf("reading the size of the store: %w", err)
		}

		size += info.Size()
	}

	return size, true, nil
}

// summarize counts what a database holds, best effort: a file that is not a
// database, or a table it lacks, is simply not counted, since a broken file is
// exactly what a clean is for and its listing must not fail.
func summarize(ctx context.Context, dir string, each database) []Held {
	reader, err := Store{dir: dir, disabled: false, readOnly: true}.openAsItIs(each.name)
	if err != nil {
		return nil
	}
	defer func() { _ = reader.Close() }()

	var holds []Held

	for _, table := range each.tables {
		count, counted := countRows(ctx, reader, table)
		if counted {
			holds = append(holds, Held{What: table.what, Count: count})
		}
	}

	return holds
}

// countRows counts a table's rows, and reports false where it cannot.
func countRows(ctx context.Context, reader *sql.DB, table heldTable) (int, bool) {
	var present bool

	err := reader.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, table.table).Scan(&present)
	if err != nil || !present {
		return 0, false
	}

	var count int

	err = reader.QueryRowContext(ctx, table.count).Scan(&count)
	if err != nil {
		return 0, false
	}

	return count, true
}

// Clean removes the cache's database from dir, with its companions, and with
// CleanAll the kept data's too. It refuses, removing nothing, a relative dir or
// anything but a plain file where a database file belongs. Every file is set
// aside before any is removed, and one that cannot be set aside puts the rest
// back, so a file another program holds open fails the clean without leaving
// half a database behind.
func Clean(dir string, scope CleanScope) error {
	if !filepath.IsAbs(dir) {
		return ErrCleanRefused
	}

	present, err := filesToClean(dir, scope)
	if err != nil {
		return err
	}

	err = setAside(present)
	if err != nil {
		return err
	}

	for _, path := range present {
		err = os.Remove(path + asideSuffix)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrNotCleaned, err)
		}
	}

	return nil
}

// filesToClean is every file a clean of scope removes that is there, refusing
// one that is not a plain file.
func filesToClean(dir string, scope CleanScope) ([]string, error) {
	names := map[CleanScope][]string{CleanCache: {dbName}, CleanAll: {dbName, keptName}}[scope]

	var present []string

	for _, name := range names {
		for _, path := range companionsOf(filepath.Join(dir, name)) {
			info, err := os.Lstat(path)

			switch {
			case errors.Is(err, fs.ErrNotExist):
				continue
			case err != nil:
				return nil, fmt.Errorf("%w: %w", ErrNotCleaned, err)
			case !info.Mode().IsRegular():
				return nil, ErrCleanRefused
			}

			present = append(present, path)
		}
	}

	return present, nil
}

// setAside renames each file aside, putting back those already moved when one
// cannot be.
func setAside(paths []string) error {
	for index, path := range paths {
		err := os.Rename(path, path+asideSuffix)
		if err != nil {
			for _, moved := range paths[:index] {
				_ = os.Rename(moved+asideSuffix, moved)
			}

			return fmt.Errorf("%w: %w", ErrNotCleaned, err)
		}
	}

	return nil
}

// companionsOf is a database's path with the write-ahead log and shared-memory
// files SQLite keeps beside it.
func companionsOf(path string) []string {
	return []string{path, path + "-wal", path + "-shm"}
}
