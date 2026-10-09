// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// An open overlay names the key context CheckKeys reads for it, so a test
// can hold what the overlay answers against what the check refuses.
func TestAnOpenOverlaysKeyContextIsOneCheckKeysReads(t *testing.T) {
	t.Parallel()

	// Arrange
	help := typing(t, newWorld().live(t, 120, 40), "?")

	// Act
	context, open := help.OverlayKeyContext()

	// Assert
	read := slices.ContainsFunc(tui.OverlayKeyContexts(), func(listed tui.KeyContext) bool {
		return listed.Name == context.Name && slices.Equal(listed.Actions, context.Actions)
	})
	if !open || !read || !slices.Contains(context.Actions, "close") {
		t.Errorf("the help's key context is %+v (open %v), want one of %+v, with close among its actions",
			context, open, tui.OverlayKeyContexts())
	}
}

func TestNoOverlayKeyContextIsLiveWhileNoOverlayIsOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	panes := newWorld().live(t, 120, 40)

	// Act
	context, open := panes.OverlayKeyContext()

	// Assert
	if open {
		t.Errorf("with no overlay open, OverlayKeyContext = %+v, want none", context)
	}
}
