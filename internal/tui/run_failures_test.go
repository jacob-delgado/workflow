// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestARunOffersOnlyPlacesThatResolveToAFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second place names a file that resolves to nothing, the way go test
	// prints one relative to its package; offering it would open an empty buffer.
	failing := failingLint()
	failing.unresolved = []string{"b.go"}

	// Act
	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// Assert
	requireScreen(t, failed.View(), "a.go:1 first")
	refuseScreen(t, failed.View(), "b.go")
}

func TestARunWithNoResolveSeamOffersEveryPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no way to resolve places, none is dropped: better to offer a jump
	// that might miss than to hide every place behind a missing seam.
	deps := failingLint().deps()
	deps.Editor.Resolve = nil
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)

	// Act
	failed := typing(t, drain(t, model, model.Init()), commitKeys("x")...)

	// Assert
	requireScreen(t, failed.View(), "a.go:1 first", "b.go:2 second")
}
