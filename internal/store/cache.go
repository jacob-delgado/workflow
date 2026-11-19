// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CachedIssue is the store's shape of a cached issue — the few non-secret fields
// a first pane needs, kept as the store's own type so it stays decoupled from the
// interface's domain type. status_category is stored beside status even though a
// Jira status maps to one category: the mapping is Jira's, not ours to derive.
type CachedIssue struct {
	Key            string
	Summary        string
	Status         string
	StatusCategory string
	Type           string
	Priority       string
}

// CachedIssues is the issue list last cached for a view of a Jira instance, in
// the tracker's order, and whether the view was cached at all — a view cached
// with no issues reports found with an empty slice, distinct from one never
// cached. A disabled store, or one with no instance, reports nothing.
func (s Store) CachedIssues(ctx context.Context, instance, view string) ([]CachedIssue, bool, error) {
	if instance == "" {
		return nil, false, nil
	}

	database, found, err := s.readFile(ctx, cacheFile())
	if err != nil || !found {
		return nil, false, err
	}
	defer func() { _ = database.Close() }()

	// The parent row records that the view was cached, even when it held no issues.
	var cached bool

	err = database.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM issue_cache WHERE instance = ? AND view = ?)`, instance, view).Scan(&cached)

	switch {
	case err != nil:
		return nil, false, fmt.Errorf("reading the cached view: %w", err)
	case !cached:
		return nil, false, nil
	}

	issues, err := readCachedIssues(ctx, database, instance, view)
	if err != nil {
		return nil, false, err
	}

	return issues, true, nil
}

// readCachedIssues reads a view's cached issues in the tracker's order.
func readCachedIssues(ctx context.Context, database *sql.DB, instance, view string) ([]CachedIssue, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT issue_key, summary, status, status_category, type, priority
			FROM cached_issue WHERE instance = ? AND view = ? ORDER BY position`, instance, view)
	if err != nil {
		return nil, fmt.Errorf("reading the cached issues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var issues []CachedIssue

	for rows.Next() {
		var issue CachedIssue

		err = rows.Scan(&issue.Key, &issue.Summary, &issue.Status,
			&issue.StatusCategory, &issue.Type, &issue.Priority)
		if err != nil {
			return nil, fmt.Errorf("reading a cached issue: %w", err)
		}

		issues = append(issues, issue)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the cached issues: %w", err)
	}

	return issues, nil
}

// CacheIssues stores issues, the list for a view of an instance as it was
// read at readAt, replacing any read no later in a single transaction so a
// re-cache never leaves a half-updated view; a list read before the one kept
// is dropped, so two reads landing out of order keep the newer. A disabled or
// read-only store caches nothing.
func (s Store) CacheIssues(ctx context.Context, instance, view string, issues []CachedIssue, readAt time.Time) error {
	if s.writesNothing() || instance == "" {
		return nil
	}

	database, err := s.openFile(ctx, cacheFile())
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	return inTransaction(ctx, database, "caching the issues", func(transaction *sql.Tx) error {
		return writeCachedIssues(ctx, transaction, instance, view, issues, readAt)
	})
}

// writeCachedIssues upserts the parent view row and replaces its child issue rows
// within a transaction, so the whole view is written at once or not at all. A
// list read before the one the row holds leaves the row and its issues as
// they are.
func writeCachedIssues(
	ctx context.Context, transaction *sql.Tx, instance, view string, issues []CachedIssue, readAt time.Time,
) error {
	var written int

	// Of two lists read at the very same nanosecond, the last written is kept.
	err := transaction.QueryRowContext(ctx,
		`INSERT INTO issue_cache (instance, view, cached_at) VALUES (?, ?, ?)
			ON CONFLICT(instance, view) DO UPDATE SET cached_at = excluded.cached_at
			WHERE excluded.cached_at >= issue_cache.cached_at
			RETURNING 1`,
		instance, view, timestamp(readAt)).Scan(&written)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("caching the view: %w", err)
	}

	_, err = transaction.ExecContext(ctx,
		`DELETE FROM cached_issue WHERE instance = ? AND view = ?`, instance, view)
	if err != nil {
		return fmt.Errorf("clearing the cached issues: %w", err)
	}

	for position, issue := range issues {
		_, err = transaction.ExecContext(ctx,
			`INSERT INTO cached_issue
				(instance, view, position, issue_key, summary, status, status_category, type, priority)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			instance, view, position, issue.Key, issue.Summary, issue.Status,
			issue.StatusCategory, issue.Type, issue.Priority)
		if err != nil {
			return fmt.Errorf("caching an issue: %w", err)
		}
	}

	return nil
}
