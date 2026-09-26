// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
	"time"
)

// Announce is one announcement made in a repository: which pull request, and the
// moment it marked — opened, its CI red, or merged. The moment is an opaque int
// the interface assigns, so the store need not know the interface's vocabulary.
type Announce struct {
	Pull   int
	Moment int
}

// RecordAnnounce remembers that a pull request was announced at a moment, so a
// later session knows not to offer announcing it again. A disabled or read-only
// store records nothing.
func (s Store) RecordAnnounce(ctx context.Context, repo string, announce Announce, now time.Time) error {
	if s.writesNothing() || repo == "" {
		return nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	_, err = database.ExecContext(
		ctx,
		`INSERT INTO announces (repo, pull, moment, announced_at) VALUES (?, ?, ?, ?)
			ON CONFLICT(repo, pull, moment) DO UPDATE SET announced_at = excluded.announced_at`,
		repo, announce.Pull, announce.Moment, timestamp(now),
	)
	if err != nil {
		return fmt.Errorf("recording the announcement: %w", err)
	}

	return nil
}

// Announces are every announcement recorded for a repository, so the interface
// can open knowing what has already been posted. A disabled store reports none.
func (s Store) Announces(ctx context.Context, repo string) ([]Announce, error) {
	if s.nothingToRead() || repo == "" {
		return nil, nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx, `SELECT pull, moment FROM announces WHERE repo = ?`, repo)
	if err != nil {
		return nil, fmt.Errorf("reading the announcements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var announces []Announce

	for rows.Next() {
		var announce Announce

		err = rows.Scan(&announce.Pull, &announce.Moment)
		if err != nil {
			return nil, fmt.Errorf("reading an announcement: %w", err)
		}

		announces = append(announces, announce)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the announcements: %w", err)
	}

	return announces, nil
}
