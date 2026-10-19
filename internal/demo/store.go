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
	}
}
