// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"testing"
)

// errListRemotes stands in for a git failure to list the remotes.
var errListRemotes = errors.New("git could not list the remotes")

// baseKeys opens the composer, moves to the base field, clears the default
// "main", types prefix, then presses tab.
func baseKeys(prefix string) []string {
	keys := append([]string{"4", "n", keyTab}, slices.Repeat([]string{keyBackspace}, len("main"))...)

	return append(append(keys, letters(prefix)...), keyTab)
}

func TestTheBaseFieldCompletesToAMatchingRemoteBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranches = []string{"develop", "release-2"}
	model := opening.live(t, 120, 40)

	// Act
	view := typing(t, model, baseKeys("dev")...).View().Content

	// Assert
	requireScreen(t, view, "base      > develop")
}

func TestTabNavigatesPastTheBaseWhenNothingCompletes(t *testing.T) {
	t.Parallel()

	// Arrange
	// No remote branches, so the tab in baseKeys has nothing to complete and must
	// move on: a letter typed after it lands in the title, not the base.
	opening := withoutPull()
	model := opening.live(t, 120, 40)

	keys := append(baseKeys("dev"), letters("Z")...)

	// Act
	view := typing(t, model, keys...).View().Content

	// Assert
	// The base is left as typed and the trailing letter did not land in it, so
	// tab moved focus on rather than being swallowed by the field.
	requireScreen(t, view, "base      > dev")
	refuseScreen(t, view, "devZ")
}

func TestTheComposerOpensWithoutARemoteBranchSeam(t *testing.T) {
	t.Parallel()

	// Arrange
	// No repository binds the seam, so it is nil; opening the composer must not
	// reach for it.
	opening := withoutPull()
	opening.noRemoteBranches = true

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n").View().Content

	// Assert
	requireScreen(t, view, "base      > "+baseName)
}

func TestAFailedRemoteBranchListingStillOpensTheComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranchesErr = errListRemotes

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n").View().Content

	// Assert
	requireScreen(t, view, "base      > "+baseName)
}
