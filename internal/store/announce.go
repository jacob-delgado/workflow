// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Announce is one announcement made in a repository: which pull request, and the
// moment it marked — opened, or merged. The moment is an opaque int
// the interface assigns, so the store need not know the interface's vocabulary.
type Announce struct {
	Pull   int
	Moment int
}

// ClaimKind is what a claim on an announcement came to.
type ClaimKind int

const (
	// ClaimHeld is a claim another took first: the announcement is being
	// posted, or was, so it is not posted again. It is the zero kind, so a
	// claim that says nothing errs toward not posting.
	ClaimHeld ClaimKind = iota
	// ClaimTaken is a claim this caller took: it, and no one else, posts.
	ClaimTaken
	// ClaimNotKept is a claim a store that keeps nothing could not take —
	// disabled, read-only, or without a repository to key it by: the post
	// goes unguarded.
	ClaimNotKept
)

// Claim is what a claim on an announcement came to, and the claim as it
// stands: its generation, one for the first and one more for each post asked
// for again, when it was taken, and whether its post was confirmed.
type Claim struct {
	Kind       ClaimKind
	Generation int
	ClaimedAt  time.Time
	Posted     bool
}

// AnnounceRecord is one announcement claimed in a repository: its generation,
// when it was claimed, and whether its post was confirmed. One never
// confirmed began and was not seen to finish.
type AnnounceRecord struct {
	Announce

	Generation int
	ClaimedAt  time.Time
	Posted     bool
}

// notKept is the claim a store that keeps nothing answers.
func notKept() Claim {
	return Claim{Kind: ClaimNotKept, Generation: 0, ClaimedAt: time.Time{}, Posted: false}
}

// ClaimAnnounce claims announce in repo before it is posted, so that of every
// session and process asking at once exactly one takes it: ClaimTaken at
// generation one, or ClaimHeld with the claim as another left it. A store that
// keeps nothing answers ClaimNotKept.
func (s Store) ClaimAnnounce(ctx context.Context, repo string, announce Announce, now time.Time) (Claim, error) {
	if s.writesNothing() || repo == "" {
		return notKept(), nil
	}

	return s.inAnnounceTx(ctx, "claiming the announcement", func(transaction *sql.Tx) (Claim, error) {
		return claimIn(ctx, transaction, repo, announce, now)
	})
}

// ReclaimAnnounce claims announce in repo again, to post it again as asked,
// only if its claim is still at generation seen: ClaimTaken at the next
// generation, its confirmed post, if any, left as it was, or ClaimHeld when
// another reclaimed it first. A claim gone, as a db-clean leaves it, is
// claimed afresh. A store that keeps nothing answers ClaimNotKept.
func (s Store) ReclaimAnnounce(
	ctx context.Context, repo string, announce Announce, seen int, now time.Time,
) (Claim, error) {
	if s.writesNothing() || repo == "" {
		return notKept(), nil
	}

	return s.inAnnounceTx(ctx, "claiming the announcement again", func(transaction *sql.Tx) (Claim, error) {
		swapped, err := affected(transaction.ExecContext(ctx,
			`UPDATE announces SET claim = claim + 1, claimed_at = ?
				WHERE repo = ? AND pull = ? AND moment = ? AND claim = ?`,
			timestamp(now), repo, announce.Pull, announce.Moment, seen))
		if err != nil {
			return Claim{}, err
		}

		if swapped {
			return Claim{Kind: ClaimTaken, Generation: seen + 1, ClaimedAt: now.UTC(), Posted: false}, nil
		}

		return claimIn(ctx, transaction, repo, announce, now)
	})
}

// ConfirmAnnounce records that the post claimed at generation went out. A
// claim since taken again, or gone, is left as it is. A store that keeps
// nothing confirms nothing.
func (s Store) ConfirmAnnounce(
	ctx context.Context, repo string, announce Announce, generation int, now time.Time,
) error {
	if s.writesNothing() || repo == "" {
		return nil
	}

	return s.announceWrite(ctx, "confirming the announcement",
		`UPDATE announces SET posted_at = ? WHERE repo = ? AND pull = ? AND moment = ? AND claim = ?`,
		timestamp(now), repo, announce.Pull, announce.Moment, generation)
}

// ReleaseAnnounce gives up the claim at generation on announce, whose post
// did not go out, so it may be offered again: only a first claim, never
// confirmed, is removed, so a release never forgets a post confirmed before
// or a claim taken again since. A store that keeps nothing releases nothing.
func (s Store) ReleaseAnnounce(ctx context.Context, repo string, announce Announce, generation int) error {
	if s.writesNothing() || repo == "" {
		return nil
	}

	return s.announceWrite(ctx, "releasing the announcement",
		`DELETE FROM announces WHERE repo = ? AND pull = ? AND moment = ? AND claim = ? AND claim = 1
			AND posted_at IS NULL`,
		repo, announce.Pull, announce.Moment, generation)
}

// RecordAnnounce remembers that a pull request was announced at a moment: it
// claims it and confirms the claim, whoever took it.
func (s Store) RecordAnnounce(ctx context.Context, repo string, announce Announce, now time.Time) error {
	claim, err := s.ClaimAnnounce(ctx, repo, announce, now)
	if err != nil {
		return err
	}

	return s.ConfirmAnnounce(ctx, repo, announce, claim.Generation, now)
}

// Announces are every announcement claimed for a repository, posted or not,
// so a surface knows what has been posted and what began and was not seen to
// finish. A row no build of the store writes — a generation below one, or a
// claim time that is not one — is left out. A disabled store reports none.
func (s Store) Announces(ctx context.Context, repo string) ([]AnnounceRecord, error) {
	if s.nothingToRead() || repo == "" {
		return nil, nil
	}

	database, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx,
		`SELECT pull, moment, claim, claimed_at, posted_at IS NOT NULL FROM announces WHERE repo = ?`, repo)
	if err != nil {
		return nil, fmt.Errorf("reading the announcements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []AnnounceRecord

	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}

		if record.Generation >= 1 && !record.ClaimedAt.IsZero() {
			records = append(records, record)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the announcements: %w", err)
	}

	return records, nil
}

// scanRecord reads one announcement row; a claim time that does not parse is
// left zero, for the caller to leave the row out.
func scanRecord(rows *sql.Rows) (AnnounceRecord, error) {
	var (
		record    AnnounceRecord
		claimedAt string
	)

	err := rows.Scan(&record.Pull, &record.Moment, &record.Generation, &claimedAt, &record.Posted)
	if err != nil {
		return AnnounceRecord{}, fmt.Errorf("reading an announcement: %w", err)
	}

	record.ClaimedAt = parsedTime(claimedAt)

	return record, nil
}

// claimIn takes the first claim on announce within transaction, or reads the
// one another took.
func claimIn(
	ctx context.Context, transaction *sql.Tx, repo string, announce Announce, now time.Time,
) (Claim, error) {
	inserted, err := affected(transaction.ExecContext(ctx,
		`INSERT INTO announces (repo, pull, moment, claim, claimed_at, posted_at) VALUES (?, ?, ?, 1, ?, NULL)
			ON CONFLICT (repo, pull, moment) DO NOTHING`,
		repo, announce.Pull, announce.Moment, timestamp(now)))
	if err != nil {
		return Claim{}, err
	}

	if inserted {
		return Claim{Kind: ClaimTaken, Generation: 1, ClaimedAt: now.UTC(), Posted: false}, nil
	}

	held := Claim{Kind: ClaimHeld, Generation: 0, ClaimedAt: time.Time{}, Posted: false}

	var claimedAt string

	err = transaction.QueryRowContext(ctx,
		`SELECT claim, claimed_at, posted_at IS NOT NULL FROM announces WHERE repo = ? AND pull = ? AND moment = ?`,
		repo, announce.Pull, announce.Moment,
	).Scan(&held.Generation, &claimedAt, &held.Posted)
	if err != nil {
		return Claim{}, fmt.Errorf("reading the claim held: %w", err)
	}

	held.ClaimedAt = parsedTime(claimedAt)

	return held, nil
}

// affected reports whether the statement whose result and error are given
// changed a row.
func affected(result sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("counting the rows changed: %w", err)
	}

	return rows == 1, nil
}

// parsedTime is the time a stored _at column holds, or the zero time for one
// that does not parse: the store is a file on disk, never trusted.
func parsedTime(stored string) time.Time {
	parsed, err := time.Parse(time.RFC3339, stored)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

// inAnnounceTx runs claim in one transaction on the store, so a claim and the
// read of the one it met see the same row, saying what was being done when it
// fails.
func (s Store) inAnnounceTx(ctx context.Context, doing string, claim func(*sql.Tx) (Claim, error)) (Claim, error) {
	database, err := s.open(ctx)
	if err != nil {
		return Claim{}, err
	}
	defer func() { _ = database.Close() }()

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, fmt.Errorf("%s: %w", doing, err)
	}

	got, err := claim(transaction)
	if err != nil {
		_ = transaction.Rollback()

		return Claim{}, fmt.Errorf("%s: %w", doing, err)
	}

	err = transaction.Commit()
	if err != nil {
		return Claim{}, fmt.Errorf("%s: %w", doing, err)
	}

	return got, nil
}

// announceWrite runs one statement on the announcements, saying what was
// being done when it fails.
func (s Store) announceWrite(ctx context.Context, doing, query string, args ...any) error {
	database, err := s.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	_, err = database.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}

	return nil
}
