// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/store"
)

// readColumn reads the one value query selects from the store's file, opened as
// any other program would open it rather than through the store's own reads.
func readColumn(t *testing.T, dir, query string) string {
	t.Helper()

	database, err := sql.Open("sqlite", filepath.Join(dir, "workflow.db"))
	if err != nil {
		t.Fatalf("opening the database to read: %v", err)
	}
	defer func() { _ = database.Close() }()

	var value string

	err = database.QueryRowContext(t.Context(), query).Scan(&value)
	if err != nil {
		t.Fatalf("reading %q: %v", query, err)
	}

	return value
}

func TestEveryTimestampIsWrittenAsRFC3339InUTC(t *testing.T) {
	t.Parallel()

	// The moment is handed over in a zone west of UTC, so a column that kept the
	// caller's zone would read back with its offset rather than as Z.
	stamped := theTime().In(time.FixedZone("UTC-6", -6*60*60))

	cases := map[string]struct {
		record func(ctx context.Context, kept store.Store) error
		query  string
	}{
		"a scope's updated_at": {
			record: func(ctx context.Context, kept store.Store) error {
				return kept.RecordScope(ctx, repo, "config", stamped)
			},
			query: `SELECT updated_at FROM scopes`,
		},
		"an announcement's announced_at": {
			record: func(ctx context.Context, kept store.Store) error {
				return kept.RecordAnnounce(ctx, repo, store.Announce{Pull: 42, Moment: 1}, stamped)
			},
			query: `SELECT announced_at FROM announces`,
		},
		"a cached view's cached_at": {
			record: func(ctx context.Context, kept store.Store) error {
				return kept.CacheIssues(ctx, instance, view, []store.CachedIssue{issue("PROJ-1")}, stamped)
			},
			query: `SELECT cached_at FROM issue_cache`,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()

			// Act
			err := testCase.record(t.Context(), store.New(dir, false))
			// Assert
			if err != nil {
				t.Fatalf("recording returned %v, want nil", err)
			}

			written := readColumn(t, dir, testCase.query)

			parsed, err := time.Parse(time.RFC3339, written)
			if err != nil || !parsed.Equal(theTime()) || parsed.Location() != time.UTC {
				t.Errorf("%s = %q (parse error %v), want %s: RFC3339, in UTC",
					name, written, err, theTime().Format(time.RFC3339))
			}
		})
	}
}
