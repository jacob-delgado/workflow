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
		Announced: func(context.Context) []loop.Announced {
			return locked(w, func() []loop.Announced { return slices.Clone(w.announced) })
		},
		Favorites: func(context.Context) ([]string, error) { return favorites(), nil },
		RecordAnnounce: func(_ context.Context, made loop.Announced) error {
			w.mu.Lock()
			defer w.mu.Unlock()

			w.announced = append(w.announced, made)

			return nil
		},
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
