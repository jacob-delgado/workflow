// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// A database on disk can fail after a statement has begun: a row that cannot be
// read after others were, a transaction its commit refuses. Each is built from
// content the store finds there, as failure_test.go's cases are.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// smallestInteger is the one integer SQLite's abs() cannot answer: its positive
// is out of range, so a row computing it fails as it is read.
const smallestInteger = -9223372036854775808

func TestAReadThatFailsPartwayReturnsNothing(t *testing.T) {
	t.Parallel()

	// SQLite reads a query's first row as the query starts, so each view here
	// answers one good row and fails on its second, while the read is under way.
	cases := map[string]struct {
		schema []statement
		call   storeCall
		step   string
	}{
		"announcements": {
			schema: []statement{
				table(`CREATE TABLE announced (repo TEXT, pull INTEGER, moment INTEGER)`),
				{
					query: `INSERT INTO announced (repo, pull, moment) VALUES (?, 42, 1), (?, ?, 2)`,
					args:  []any{repo, repo, smallestInteger},
				},
				table(`CREATE VIEW announces AS SELECT repo, abs(pull) AS pull, moment FROM announced`),
			},
			call: announces,
			step: "reading the announcements",
		},
		"cached issues": {
			schema: append(cachedView(),
				table(`CREATE TABLE listed (
					instance TEXT, view TEXT, position INTEGER, number INTEGER,
					PRIMARY KEY (instance, view, position)
				)`),
				statement{
					query: `INSERT INTO listed (instance, view, position, number) VALUES (?, ?, 0, 1), (?, ?, 1, ?)`,
					args:  []any{instance, view, instance, view, smallestInteger},
				},
				table(`CREATE VIEW cached_issue AS SELECT instance, view, position,
					'PROJ-' || abs(number) AS issue_key, 'summary' AS summary, 'To Do' AS status,
					'new' AS status_category, 'Task' AS type, 'High' AS priority FROM listed`),
			),
			call: cachedIssues,
			step: "reading the cached issues",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			seedDatabase(t, dir, testCase.schema...)

			// Act
			readBack, err := testCase.call(t.Context(), store.New(dir, false))

			// Assert
			if err == nil || !strings.Contains(err.Error(), testCase.step) || readBack {
				t.Errorf("err = %v, read back %v; want the failed read reported by %q and the rows before it dropped",
					err, readBack, testCase.step)
			}
		})
	}
}

func TestACacheItsCommitRefusesIsNotKept(t *testing.T) {
	t.Parallel()

	// Arrange
	// The cached issues' table as found on disk checks its parent only as the
	// transaction commits, and names a parent table that holds no row: every
	// issue is written, and the commit refuses them all.
	dir := t.TempDir()
	seedDatabase(t, dir,
		table(`CREATE TABLE no_views (
			instance TEXT NOT NULL, view TEXT NOT NULL, PRIMARY KEY (instance, view)
		) STRICT`),
		table(`CREATE TABLE cached_issue (
			instance TEXT NOT NULL, view TEXT NOT NULL, position INTEGER NOT NULL,
			issue_key TEXT NOT NULL, summary TEXT NOT NULL, status TEXT NOT NULL,
			status_category TEXT NOT NULL, type TEXT NOT NULL, priority TEXT NOT NULL,
			PRIMARY KEY (instance, view, position),
			FOREIGN KEY (instance, view) REFERENCES no_views(instance, view) DEFERRABLE INITIALLY DEFERRED
		) STRICT`),
	)

	kept := store.New(dir, false)

	// Act
	err := kept.CacheIssues(t.Context(), instance, view, []store.CachedIssue{issue("PROJ-1")}, theTime())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "caching the issues") {
		t.Errorf("CacheIssues = %v, want the refused commit reported", err)
	}

	got, found, readErr := kept.CachedIssues(t.Context(), instance, view)
	if readErr != nil || found || len(got) != 0 {
		t.Errorf("CachedIssues after the refused commit = %v, %v, %v; want the view not cached", got, found, readErr)
	}
}
