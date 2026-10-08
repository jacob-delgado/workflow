// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// questionedDir is a store directory whose name holds a question mark, which
// in a database's address starts its parameters, beside nothing else in its
// parent.
func questionedDir(t *testing.T) (string, string) {
	t.Helper()

	parent := t.TempDir()

	return parent, filepath.Join(parent, "a?b")
}

// onlyEntry fails the test unless parent holds dir alone.
func onlyEntry(t *testing.T, parent, dir string) {
	t.Helper()

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("listing %s: %v", parent, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	if !slices.Equal(names, []string{filepath.Base(dir)}) {
		t.Errorf("%s holds %q, want the store's directory alone", parent, names)
	}
}

func TestAStoreDirectoryWithAQuestionMarkKeepsTheCacheInsideWithForeignKeysOn(t *testing.T) {
	t.Parallel()

	// Arrange
	// The cached issues' table names a parent table that holds no row, so
	// only a connection that enforces foreign keys refuses an issue.
	parent, dir := questionedDir(t)
	seeded := filepath.Join(parent, "seeded")

	err := os.Mkdir(seeded, 0o700)
	if err != nil {
		t.Fatalf("making the directory to seed: %v", err)
	}

	seedDatabase(t, seeded,
		table(`CREATE TABLE no_views (
			instance TEXT NOT NULL, view TEXT NOT NULL, PRIMARY KEY (instance, view)
		) STRICT`),
		table(`CREATE TABLE cached_issue (
			instance TEXT NOT NULL, view TEXT NOT NULL, position INTEGER NOT NULL,
			issue_key TEXT NOT NULL, summary TEXT NOT NULL, status TEXT NOT NULL,
			status_category TEXT NOT NULL, type TEXT NOT NULL, priority TEXT NOT NULL,
			PRIMARY KEY (instance, view, position),
			FOREIGN KEY (instance, view) REFERENCES no_views(instance, view) ON DELETE CASCADE
		) STRICT`),
	)

	err = os.Rename(seeded, dir)
	if err != nil {
		t.Fatalf("naming the directory with a question mark: %v", err)
	}

	// Act
	_, err = cacheIssues(t.Context(), store.New(dir, false))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "caching an issue") {
		t.Errorf("CacheIssues = %v, want the seeded file inside the directory, refusing an orphan", err)
	}

	onlyEntry(t, parent, dir)
}

func TestAStoreDirectoryWithAQuestionMarkKeepsTheKeptDataInside(t *testing.T) {
	t.Parallel()

	// Arrange
	parent, dir := questionedDir(t)
	kept := store.New(dir, false)

	// Act
	linkAna(t, kept)

	// Assert
	_, err := os.Stat(filepath.Join(dir, "kept.db"))
	if err != nil {
		t.Errorf("the kept data is not inside the store's directory: %v", err)
	}

	onlyEntry(t, parent, dir)
}
