// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"slices"

	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/seams"
)

// storeDeps is the demo's on-disk store: the pull requests announced, and the
// directories kept as favorites.
func (w *World) storeDeps() seams.Store {
	return seams.Store{
		Announced: func(context.Context) ([]loop.AnnounceRecord, error) {
			return locked(w, w.announcedRecords), nil
		},
		Favorites:     func(context.Context) ([]string, error) { return favorites(), nil },
		ClaimAnnounce: w.claimAnnounce,
		ConfirmAnnounce: func(context.Context, loop.Announced, int) error {
			return nil
		},
		ReleaseAnnounce: w.releaseAnnounce,
		LastScope: func(context.Context) (string, bool) {
			return locked(w, func() string { return w.scope }), true
		},
		RecordScope: func(_ context.Context, scope string) error {
			w.mu.Lock()
			defer w.mu.Unlock()

			w.scope = scope

			return nil
		},
	}
}

// announcedRecords is every announcement the demo claimed, each posted. The
// caller holds the lock.
func (w *World) announcedRecords() []loop.AnnounceRecord {
	records := make([]loop.AnnounceRecord, 0, len(w.announced))
	for _, made := range w.announced {
		records = append(records, loop.AnnounceRecord{Made: made, Generation: 1, ClaimedAt: epoch(), Posted: true})
	}

	return records
}

// claimAnnounce claims made, as the store does: once, the first to ask.
func (w *World) claimAnnounce(_ context.Context, made loop.Announced) (loop.Claim, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if slices.Contains(w.announced, made) {
		return loop.Claim{Kind: loop.ClaimHeld, Generation: 1, ClaimedAt: epoch(), Posted: true}, nil
	}

	w.announced = append(w.announced, made)

	return loop.Claim{Kind: loop.ClaimTaken, Generation: 1, ClaimedAt: epoch(), Posted: false}, nil
}

// releaseAnnounce gives up the demo's claim on made, whose post did not go.
func (w *World) releaseAnnounce(_ context.Context, made loop.Announced, _ int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.announced = slices.DeleteFunc(w.announced, func(claimed loop.Announced) bool { return claimed == made })

	return nil
}
