// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The store's directory and file live outside the process, so these tests hand
// it what it may find there instead of what it made: a path it cannot create, a
// file that is not a database, and a schema someone changed. Each is built from
// content, never from permissions, so it fails the same way for every user,
// root included.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// view is the issue view the failure tests cache and read.
const view = "assigned"

// storeCall calls one store method, and reports what it returned: whether
// anything was read back, and its error.
type storeCall func(ctx context.Context, kept store.Store) (readBack bool, err error)

func lastScope(ctx context.Context, kept store.Store) (bool, error) {
	scope, found, err := kept.LastScope(ctx, repo)

	return found || scope != "", err
}

func recordScope(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.RecordScope(ctx, repo, "config", theTime())
}

func recordAnnounce(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.RecordAnnounce(ctx, repo, store.Announce{Pull: 42, Moment: 1}, theTime())
}

func announces(ctx context.Context, kept store.Store) (bool, error) {
	got, err := kept.Announces(ctx, repo)

	return len(got) > 0, err
}

func cachedIssues(ctx context.Context, kept store.Store) (bool, error) {
	got, found, err := kept.CachedIssues(ctx, instance, view)

	return found || len(got) > 0, err
}

func cacheIssues(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.CacheIssues(ctx, instance, view, []store.CachedIssue{issue("PROJ-1")}, theTime())
}

// statement is one SQL statement run into a database before the store first
// opens it, with the values bound to its placeholders.
type statement struct {
	query string
	args  []any
}

// seedDatabase leaves a database in dir for the store to find, as if an older
// version or another program had written it: each statement runs in order.
func seedDatabase(t *testing.T, dir string, statements ...statement) {
	t.Helper()

	database, err := sql.Open("sqlite", filepath.Join(dir, "workflow.db"))
	if err != nil {
		t.Fatalf("opening the database to seed: %v", err)
	}
	defer func() { _ = database.Close() }()

	for _, each := range statements {
		_, err = database.ExecContext(t.Context(), each.query, each.args...)
		if err != nil {
			t.Fatalf("seeding %q: %v", each.query, err)
		}
	}
}

// table is a statement that creates a table and binds nothing.
func table(query string) statement {
	return statement{query: query, args: nil}
}

// cachedView is the cache's real parent table holding the view the tests read,
// so a read gets past the parent to whatever is wrong with its issues.
func cachedView() []statement {
	return []statement{
		table(`CREATE TABLE issue_cache (
			instance TEXT NOT NULL, view TEXT NOT NULL, cached_at TEXT NOT NULL, PRIMARY KEY (instance, view)
		) STRICT`),
		{
			query: `INSERT INTO issue_cache (instance, view, cached_at) VALUES (?, ?, ?)`,
			args:  []any{instance, view, "2026-09-22T12:00:00Z"},
		},
	}
}

func TestEveryStoreMethodReportsADirectoryItCannotCreate(t *testing.T) {
	t.Parallel()

	cases := map[string]storeCall{
		"LastScope": lastScope, "RecordScope": recordScope,
		"RecordAnnounce": recordAnnounce, "Announces": announces,
		"CachedIssues": cachedIssues, "CacheIssues": cacheIssues,
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A regular file where the store directory's parent should be: no
			// directory can be made inside a file, whoever asks.
			parent := filepath.Join(t.TempDir(), "not-a-directory")

			err := os.WriteFile(parent, []byte("a file"), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			kept := store.New(filepath.Join(parent, "store"), false)

			// Act
			readBack, err := call(t.Context(), kept)

			// Assert
			if !errors.Is(err, syscall.ENOTDIR) || readBack {
				t.Errorf("%s = %v, read back %v; want the directory it could not create reported and nothing read",
					name, err, readBack)
			}
		})
	}
}

func TestAFileThatIsNotADatabaseIsReportedAndLeftAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.db")
	notADatabase := bytes.Repeat([]byte("this is not a database\n"), 45)

	err := os.WriteFile(path, notADatabase, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = store.New(dir, false).RecordScope(t.Context(), repo, "config", theTime())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "preparing the store schema") {
		t.Errorf("RecordScope over a file that is not a database = %v, want the schema step to report it", err)
	}

	left, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(left, notADatabase) {
		t.Errorf("the file now holds %d bytes (err %v), want its %d bytes left as they were",
			len(left), readErr, len(notADatabase))
	}
}

func TestATamperedSchemaIsReportedNotTrusted(t *testing.T) {
	t.Parallel()

	// Each table exists already, so the store's CREATE TABLE IF NOT EXISTS keeps
	// it as found: a missing column fails the statement that names it, and a
	// table that is not STRICT hands back a value of the wrong type.
	scopesWithoutColumns := table(`CREATE TABLE scopes (other TEXT)`)
	announcesWithoutColumns := table(`CREATE TABLE announces (other TEXT)`)
	cacheWithoutColumns := table(`CREATE TABLE issue_cache (other TEXT)`)
	issuesWithoutColumns := table(`CREATE TABLE cached_issue (other TEXT)`)
	issuesWithOnlyTheirView := table(`CREATE TABLE cached_issue (instance TEXT, view TEXT)`)

	cases := map[string]struct {
		schema []statement
		call   storeCall
		// step is the store's own words for the statement that meets the
		// tampered table, so a later statement failing instead does not pass
		// for it.
		step string
	}{
		"a scope read from a table without its columns": {
			schema: []statement{scopesWithoutColumns}, call: lastScope, step: "reading the last scope",
		},
		"a scope written to a table without its columns": {
			schema: []statement{scopesWithoutColumns}, call: recordScope, step: "recording the scope",
		},
		"an announcement written to a table without its columns": {
			schema: []statement{announcesWithoutColumns}, call: recordAnnounce, step: "recording the announcement",
		},
		"announcements read from a table without their columns": {
			schema: []statement{announcesWithoutColumns}, call: announces, step: "reading the announcements",
		},
		"an announcement whose pull request is not a number": {
			schema: []statement{
				table(`CREATE TABLE announces (repo TEXT, pull, moment, announced_at TEXT)`),
				{
					query: `INSERT INTO announces (repo, pull, moment, announced_at) VALUES (?, ?, ?, ?)`,
					args:  []any{repo, "seven", 1, "2026-09-22T12:00:00Z"},
				},
			},
			call: announces,
			step: "reading an announcement",
		},
		"a view read from a table without its columns": {
			schema: []statement{cacheWithoutColumns}, call: cachedIssues, step: "reading the cached view",
		},
		"a view written to a table without its columns": {
			schema: []statement{cacheWithoutColumns}, call: cacheIssues, step: "caching the view",
		},
		"issues cleared from a table without their columns": {
			schema: append(cachedView(), issuesWithoutColumns), call: cacheIssues, step: "clearing the cached issues",
		},
		"an issue written to a table with only its view": {
			schema: append(cachedView(), issuesWithOnlyTheirView), call: cacheIssues, step: "caching an issue",
		},
		"issues read from a table with only their view": {
			schema: append(cachedView(), issuesWithOnlyTheirView), call: cachedIssues, step: "reading the cached issues",
		},
		"a cached issue with no summary": {
			schema: append(
				cachedView(),
				table(`CREATE TABLE cached_issue (
					instance TEXT, view TEXT, position INTEGER, issue_key TEXT, summary TEXT,
					status TEXT, status_category TEXT, type TEXT, priority TEXT
				)`),
				statement{
					query: `INSERT INTO cached_issue (instance, view, position, issue_key) VALUES (?, ?, ?, ?)`,
					args:  []any{instance, view, 0, "PROJ-1"},
				},
			),
			call: cachedIssues,
			step: "reading a cached issue",
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
				t.Errorf("err = %v, read back %v; want the tampered table reported by %q and nothing read",
					err, readBack, testCase.step)
			}
		})
	}
}
