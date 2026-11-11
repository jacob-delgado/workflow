// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// otherFeatureBranch is a feature branch checked out after a draft was kept
// on the world's fix branch.
const otherFeatureBranch = "feat/PROJ-388-add-retries"

func TestAKeptDraftStaysWithItsBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	left := typing(t, repo.live(t, 120, 40), append(append([]string{"3", "c"}, letters("half done")...), keyEsc)...)
	repo.branch.Name = otherFeatureBranch
	switched := typing(t, left, "2", "r")

	// Act
	view := typing(t, switched, "3", "c").View().Content

	// Assert
	requireScreen(t, view, "‹feat›")
	refuseScreen(t, view, "‹fix›", "half done")
}

func TestAKeptDraftComesBackOnItsBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	left := typing(t, repo.live(t, 120, 40), append(append([]string{"3", "c"}, letters("half done")...), keyEsc)...)
	repo.branch.Name = otherFeatureBranch
	away := typing(t, left, "2", "r")
	repo.branch.Name = featureName
	back := typing(t, away, "2", "r")

	// Act
	view := typing(t, back, "3", "c").View().Content

	// Assert
	requireScreen(t, view, "‹fix›", "subject > half done")
}

// keptOnTheFixBranch is the world's interface with "half done" written in the
// composer on the fix branch and left with esc, then the feature branch
// checked out.
func keptOnTheFixBranch(t *testing.T, repo *world) tui.Model {
	t.Helper()

	left := typing(t, repo.live(t, 120, 40), append(append([]string{"3", "c"}, letters("half done")...), keyEsc)...)
	repo.branch.Name = otherFeatureBranch

	return typing(t, left, "2", "r")
}

func TestAKeptDraftOutlivesTheComposerLookedAtOnAnotherBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	looked := typing(t, keptOnTheFixBranch(t, repo), "3", "c", keyEsc)
	repo.branch.Name = featureName
	back := typing(t, looked, "2", "r")

	// Act
	view := typing(t, back, "3", "c").View().Content

	// Assert
	requireScreen(t, view, "‹fix›", "subject > half done")
}

func TestAKeptDraftOutlivesACommitOnAnotherBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	committed := typing(t, keptOnTheFixBranch(t, repo), commitKeys("x")...)
	repo.branch.Name = featureName
	back := typing(t, committed, "2", "r")

	// Act
	view := typing(t, back, "3", "c").View().Content

	// Assert
	requireScreen(t, view, "‹fix›", "subject > half done")
}

func TestASwitchNamesADraftKeptOnAnotherBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withDirectories(newWorld())
	looked := typing(t, keptOnTheFixBranch(t, repo), "3", "c", keyEsc)

	// Act
	view := switchingToWeb(t, looked)

	// Assert
	requireScreen(t, view, "Switch to ~/src/web", "the commit message you were writing")
}
