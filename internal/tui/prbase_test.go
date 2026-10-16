// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
)

// errListRemotes stands in for a git failure to list the remotes.
var errListRemotes = errors.New("git could not list the remotes")

// baseKeys opens the composer on its base field, clears the default "main",
// types prefix, then presses tab.
func baseKeys(prefix string) []string {
	return []string{"4", "n", keyClearLine, pasted(prefix), keyTab}
}

func TestTheBaseFieldCompletesToAMatchingRemoteBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranches = []string{"develop", "release-2"}
	model := opening.live(t, 120, 40)

	// Act
	view := typing(t, model, append(baseKeys("dev"), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "→ develop")
}

// The base the composer opens at stays the base when the field is left
// untouched but for a key that types nothing, though a remote branch begins
// with its name and none is it, as when the base was never pushed.
func TestTheBaseTheComposerOpensAtIsKeptThoughANewerBranchBeginsWithIt(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranches = []string{baseName + "-backport"}
	model := opening.live(t, 120, 40)

	// Act
	view := typing(t, model, "4", "n", keyEnd, keyTab, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Title", "→ "+baseName)
	refuseScreen(t, view, "→ "+baseName+"-backport")
}

// A base typed whole is the base, though a newer remote branch, listed
// first, begins with its name.
func TestABaseTypedWholeIsKeptThoughANewerBranchBeginsWithIt(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranches = []string{baseName + "-backport", baseName}
	model := opening.live(t, 120, 40)

	// Act
	view := typing(t, model, append(baseKeys(baseName), keyEnter)...).View().Content

	// Assert
	requireScreen(t, view, "Title", "→ "+baseName)
	refuseScreen(t, view, "→ "+baseName+"-backport")
}

func TestTabMovesPastTheBaseWhenNothingCompletes(t *testing.T) {
	t.Parallel()

	// Arrange
	// No remote branches, so the tab in baseKeys has nothing to complete and
	// moves on to the template: a letter typed after it does not land in the
	// base.
	keys := append(baseKeys("dev"), "Z")

	// Act
	view := typing(t, twoTemplates().live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, "> dev")
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
	requireScreen(t, view, "> "+baseName)
}

func TestAFailedRemoteBranchListingStillOpensTheComposer(t *testing.T) {
	t.Parallel()

	// Arrange
	opening := withoutPull()
	opening.remoteBranchesErr = errListRemotes

	// Act
	view := typing(t, opening.live(t, 120, 40), "4", "n").View().Content

	// Assert
	requireScreen(t, view, "> "+baseName)
}
