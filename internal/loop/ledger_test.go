// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/loop"
)

// claimedAt is when the ledger's claims are taken.
func claimedAt() time.Time {
	return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)
}

// ledger is a store's claims on announcements, kept as the store keeps them:
// one claim per announcement, taken by the first to ask, confirmed once its
// post went out, given up only while it is a first claim never confirmed,
// and taken again only from the generation seen. Its seams fail with the
// errors set, and count what each was asked.
type ledger struct {
	mu         sync.Mutex
	records    []loop.AnnounceRecord
	claims     int
	claimErr   error
	confirmErr error
	releaseErr error
}

// memory is the ledger as the memory a delivery claims, confirms and releases
// through.
func (l *ledger) memory() loop.AnnounceMemory {
	return loop.AnnounceMemory{
		Recorded:     l.recorded,
		Claim:        l.claim,
		Confirm:      l.confirm,
		Release:      l.release,
		Reclaim:      l.reclaim,
		RecordGroups: nil,
	}
}

// recorded is every claim kept.
func (l *ledger) recorded(context.Context) ([]loop.AnnounceRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]loop.AnnounceRecord(nil), l.records...), nil
}

// claim takes the first claim on made, or answers the one held.
func (l *ledger) claim(_ context.Context, made loop.Announced) (loop.Claim, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.claims++

	if l.claimErr != nil {
		return loop.Claim{}, l.claimErr
	}

	return l.claimLocked(made), nil
}

// claimLocked is claim, the lock held.
func (l *ledger) claimLocked(made loop.Announced) loop.Claim {
	record, held := loop.RecordOf(l.records, made)
	if held {
		return loop.Claim{
			Kind: loop.ClaimHeld, Generation: record.Generation, ClaimedAt: record.ClaimedAt, Posted: record.Posted,
		}
	}

	l.records = append(l.records, loop.AnnounceRecord{Made: made, Generation: 1, ClaimedAt: claimedAt(), Posted: false})

	return loop.Claim{Kind: loop.ClaimTaken, Generation: 1, ClaimedAt: claimedAt(), Posted: false}
}

// reclaim takes made's claim again from generation seen.
func (l *ledger) reclaim(_ context.Context, made loop.Announced, seen int) (loop.Claim, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.claims++

	for index, record := range l.records {
		if record.Made == made && record.Generation == seen {
			l.records[index].Generation++

			return loop.Claim{Kind: loop.ClaimTaken, Generation: seen + 1, ClaimedAt: claimedAt(), Posted: false}, nil
		}
	}

	return l.claimLocked(made), nil
}

// confirm marks made's claim at generation posted.
func (l *ledger) confirm(_ context.Context, made loop.Announced, generation int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.confirmErr != nil {
		return l.confirmErr
	}

	for index, record := range l.records {
		if record.Made == made && record.Generation == generation {
			l.records[index].Posted = true
		}
	}

	return nil
}

// release gives up made's first claim at generation, never confirmed.
func (l *ledger) release(_ context.Context, made loop.Announced, generation int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.releaseErr != nil {
		return l.releaseErr
	}

	for index, record := range l.records {
		if record.Made == made && record.Generation == generation && generation == 1 && !record.Posted {
			l.records = append(l.records[:index], l.records[index+1:]...)

			break
		}
	}

	return nil
}

// standing is how made stands in the ledger: unclaimed, claimed and
// unconfirmed, or posted.
func (l *ledger) standing(made loop.Announced) loop.Standing {
	l.mu.Lock()
	defer l.mu.Unlock()

	record, claimed := loop.RecordOf(l.records, made)

	switch {
	case !claimed:
		return loop.StandingNone
	case record.Posted:
		return loop.StandingPosted
	default:
		return loop.StandingUnconfirmed
	}
}
