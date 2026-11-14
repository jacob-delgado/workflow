// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The Repositories pane reads the worktrees and the favorites when it is
// first looked at, and again after a change to either: a favorite marked or
// forgotten, a worktree made. It lists what the read it began last found.

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// madeWorktreeDir is where the branch creator makes the feature branch's
// worktree.
const madeWorktreeDir = "/work-" + featureName

func TestAWorktreeMadeWhileTheRepositoriesAreFirstReadIsListed(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pane's first read is slow; a worktree is made for the feature
	// branch meanwhile, and the first read, begun before it, answers last.
	working := worktreesWorld()
	opened, late := withheldKeys(t, working.live(t, 120, 40), working.counting("favorites"), reposKey)
	working.dirs.worktrees = append(slices.Clone(working.dirs.worktrees),
		gitrepo.Worktree{Dir: madeWorktreeDir, Branch: featureName, Head: worktreeHead})
	made := handedOver(t, typing(t, opened, "2", "b", keyWorktree, keyEnter, keyEsc), late)

	// Act
	view := typing(t, made, reposKey).View().Content

	// Assert
	requireScreen(t, view, madeWorktreeDir, "~/src/api-feat-x")
}

func TestFPressedAgainBeforeTheFavoritesAreReadAgainChangesThemOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	// f marks where you work a favorite; the change answers, and the read
	// after it, which would show the star, is slow.
	working := reposWorld()
	opened := typing(t, working.live(t, 120, 40), reposKey)
	marked, _ := withheldKeys(t, opened, working.counting("favorites"), "f")

	// Act
	view := typing(t, marked, "f").View().Content

	// Assert
	requireScreen(t, view, "the favorites are being read")

	if changed := working.asked("favor "); len(changed) != 1 {
		t.Errorf("marked %q, want one change while the read after it was out", changed)
	}
}

func TestAnOlderRepositoriesReadAnsweringLateIsNotShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the favorites, slowly, with ~/src/web among them; it
	// is forgotten in another session, and a second refresh answers first.
	working := reposWorld()
	slow, late := withheldKeys(t, typing(t, working.live(t, 120, 40), reposKey), working.counting("favorites"), "r")
	working.dirs.favorites = []string{oldDir}

	refreshed := typing(t, slow, "r")

	// Act
	view := handedOver(t, refreshed, late).View().Content

	// Assert
	refuseScreen(t, view, "★ ~/src/web")
}
