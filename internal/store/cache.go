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
	if s.nothingToRead() || instance == "" {
		return nil, false, nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = database.Close() }()

	// The parent row records that the view was cached, even when it held no issues.
	var cachedAt string

	err = database.QueryRowContext(ctx,
		`SELECT cached_at FROM issue_cache WHERE instance = ? AND view = ?`, instance, view).Scan(&cachedAt)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("reading the cached view: %w", err)
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

// CacheIssues stores issues as the list last seen for a view of an instance,
// replacing any earlier one in a single transaction so a re-cache never leaves a
// half-updated view. A disabled or read-only store caches nothing.
func (s Store) CacheIssues(ctx context.Context, instance, view string, issues []CachedIssue, now time.Time) error {
	if s.writesNothing() || instance == "" {
		return nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("caching the issues: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	err = writeCachedIssues(ctx, transaction, instance, view, issues, now)
	if err != nil {
		return err
	}

	err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("caching the issues: %w", err)
	}

	return nil
}

// writeCachedIssues upserts the parent view row and replaces its child issue rows
// within a transaction, so the whole view is written at once or not at all.
func writeCachedIssues(
	ctx context.Context, transaction *sql.Tx, instance, view string, issues []CachedIssue, now time.Time,
) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO issue_cache (instance, view, cached_at) VALUES (?, ?, ?)
			ON CONFLICT(instance, view) DO UPDATE SET cached_at = excluded.cached_at`,
		instance, view, timestamp(now))
	if err != nil {
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
