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

// CachedIssues is the last issue list stored for a view of a Jira instance, as
// the opaque payload it was cached with, so the interface can show it at once
// before the tracker answers. It reports no payload when nothing is cached, when
// the store is disabled, or when there is no instance to key by.
func (s Store) CachedIssues(ctx context.Context, instance, view string) ([]byte, bool, error) {
	if s.off() || instance == "" {
		return nil, false, nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = database.Close() }()

	var payload []byte

	err = database.QueryRowContext(ctx,
		`SELECT payload FROM issue_cache WHERE instance = ? AND view = ?`, instance, view).Scan(&payload)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("reading the cached issues: %w", err)
	default:
		return payload, true, nil
	}
}

// CacheIssues stores payload as the issue list last seen for a view of an
// instance, replacing any earlier one. A disabled store caches nothing.
func (s Store) CacheIssues(ctx context.Context, instance, view string, payload []byte, now time.Time) error {
	if s.off() || instance == "" {
		return nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	_, err = database.ExecContext(
		ctx,
		`INSERT INTO issue_cache (instance, view, payload, cached_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(instance, view) DO UPDATE SET payload = excluded.payload, cached_at = excluded.cached_at`,
		instance, view, payload, now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("caching the issues: %w", err)
	}

	return nil
}
