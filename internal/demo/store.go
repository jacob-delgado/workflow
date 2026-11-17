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
		Favorites:       func(context.Context) ([]string, error) { return favorites(), nil },
		ClaimAnnounce:   w.claimAnnounce,
		ConfirmAnnounce: w.confirmAnnounce,
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

// claimState is how far a demo claim has got: taken, or its post confirmed.
type claimState int

const (
	claimTaken claimState = iota
	claimPosted
)

// demoClaim is one announcement the demo claimed, and how far it has got.
type demoClaim struct {
	made  loop.Announced
	state claimState
}

// announcedRecords is every announcement the demo claimed, posted once its
// post was confirmed. The caller holds the lock.
func (w *World) announcedRecords() []loop.AnnounceRecord {
	records := make([]loop.AnnounceRecord, 0, len(w.announced))
	for _, claim := range w.announced {
		records = append(records, loop.AnnounceRecord{
			Made: claim.made, Generation: 1, ClaimedAt: epoch(), Posted: claim.state == claimPosted,
		})
	}

	return records
}

// claimAnnounce claims made, as the store does: once, the first to ask. A
// claim already held says whether its post was confirmed.
func (w *World) claimAnnounce(_ context.Context, made loop.Announced) (loop.Claim, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if index := w.claimOn(made); index >= 0 {
		posted := w.announced[index].state == claimPosted

		return loop.Claim{Kind: loop.ClaimHeld, Generation: 1, ClaimedAt: epoch(), Posted: posted}, nil
	}

	w.announced = append(w.announced, demoClaim{made: made, state: claimTaken})

	return loop.Claim{Kind: loop.ClaimTaken, Generation: 1, ClaimedAt: epoch(), Posted: false}, nil
}

// confirmAnnounce records that made's post went. A claim the demo does not
// hold is left alone, as the store leaves a row it does not find; the demo
// hands out one generation and compares none.
func (w *World) confirmAnnounce(_ context.Context, made loop.Announced, _ int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if index := w.claimOn(made); index >= 0 {
		w.announced[index].state = claimPosted
	}

	return nil
}

// releaseAnnounce gives up the demo's claim on made, whose post did not go.
// A claim whose post was confirmed is kept, as the store never gives up a
// confirmed post.
func (w *World) releaseAnnounce(_ context.Context, made loop.Announced, _ int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.announced = slices.DeleteFunc(w.announced, func(claim demoClaim) bool {
		return claim.made == made && claim.state == claimTaken
	})

	return nil
}

// claimOn is where the demo's claim on made is among those it holds, or -1
// where it holds none. The caller holds the lock.
func (w *World) claimOn(made loop.Announced) int {
	return slices.IndexFunc(w.announced, func(claim demoClaim) bool { return claim.made == made })
}
