// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotADirectoryPath is a favorite that is not an absolute, clean path.
var ErrNotADirectoryPath = errors.New("a favorite is an absolute path to a directory, written plainly")

// favoritesSchema makes the favorite directories table: each directory you
// marked, and when. Whether it is still there, or a repository, is read from
// the disk each time, never kept.
func favoritesSchema() []string {
	return []string{
		`CREATE TABLE favorite_dir (
			dir      TEXT NOT NULL PRIMARY KEY,
			added_at TEXT NOT NULL
		) STRICT`,
	}
}

// Favorite is a directory you marked, and when.
type Favorite struct {
	Dir   string
	Added time.Time
}

// Favorites is the directories you marked, by path. A row of the wrong shape
// is left out. A disabled store, or one with none, reports none.
func (s Store) Favorites(ctx context.Context) ([]Favorite, error) {
	database, found, err := s.readKept(ctx)
	if err != nil || !found {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx, `SELECT dir, added_at FROM favorite_dir ORDER BY dir`)
	if err != nil {
		return nil, fmt.Errorf("reading the favorite directories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var favorites []Favorite

	for rows.Next() {
		favorite, ok, err := scanFavorite(rows)
		if err != nil {
			return nil, err
		}

		if ok {
			favorites = append(favorites, favorite)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the favorite directories: %w", err)
	}

	return favorites, nil
}

// scanFavorite reads one row, and whether it is the shape Favor writes.
func scanFavorite(rows *sql.Rows) (Favorite, bool, error) {
	var dir, added string

	err := rows.Scan(&dir, &added)
	if err != nil {
		return Favorite{}, false, fmt.Errorf("reading a favorite directory: %w", err)
	}

	at, parseErr := time.Parse(time.RFC3339, added)

	return Favorite{Dir: dir, Added: at.UTC()}, parseErr == nil && plainDirectory(dir), nil
}

// Favor marks dir a favorite, keeping when it was first marked. A disabled or
// read-only store records nothing.
func (s Store) Favor(ctx context.Context, dir string, now time.Time) error {
	if !plainDirectory(dir) {
		return ErrNotADirectoryPath
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx,
			`INSERT INTO favorite_dir (dir, added_at) VALUES (?, ?) ON CONFLICT(dir) DO NOTHING`,
			dir, timestamp(now))
		if err != nil {
			return fmt.Errorf("marking a favorite directory: %w", err)
		}

		return nil
	})
}

// Unfavor forgets dir as a favorite. A disabled or read-only store changes
// nothing.
func (s Store) Unfavor(ctx context.Context, dir string) error {
	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx, `DELETE FROM favorite_dir WHERE dir = ?`, dir)
		if err != nil {
			return fmt.Errorf("forgetting a favorite directory: %w", err)
		}

		return nil
	})
}

// plainDirectory reports dir an absolute path written as filepath.Clean writes
// it, with no NUL a path cannot hold.
func plainDirectory(dir string) bool {
	return filepath.IsAbs(dir) && filepath.Clean(dir) == dir && !strings.ContainsRune(dir, 0)
}
