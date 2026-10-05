// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The api repository's other worktrees, beside its root.
const (
	apiFeature   = "/home/ana/src/api-feat-x"
	apiReview    = "/home/ana/src/api-review"
	apiGone      = "/home/ana/src/api-gone"
	worktreeHead = "300a7be68d29eb9302518fd80f1cf20267d6feba"
)

var errWorktreesUnread = errors.New("git worktree list failed")

// worktreesWorld is reposWorld with api's worktrees: its root on main, one
// on a feature branch, one detached and locked, and one whose directory is
// gone.
func worktreesWorld() *world {
	working := reposWorld()
	working.dirs.worktrees = []gitrepo.Worktree{
		{Dir: apiRoot, Branch: "main", Head: worktreeHead},
		{Dir: apiFeature, Branch: "feat/x", Head: worktreeHead},
		{Dir: apiReview, Head: worktreeHead, Detached: true, Locked: true},
		{Dir: apiGone, Branch: "gone", Head: worktreeHead, Missing: true},
	}

	return working
}

func TestTheRepositoriesPaneListsTheOtherWorktrees(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, worktreesWorld().live(t, 120, 40), reposKey).View().Content

	// Assert
	// The worktree here is where you work, already said, so it is not listed
	// again.
	requireScreen(t, view, "Worktrees", "~/src/api-feat-x", "worktree on feat/x",
		"~/src/api-review", "worktree at 300a7be, locked", "~/src/api-gone", "worktree gone")

	if strings.Contains(view, "worktree on main") {
		t.Errorf("the worktree here is listed again:\n%s", view)
	}
}

func TestEnterOnAWorktreeLeavesForIt(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, worktreesWorld().live(t, 120, 40), reposKey, "j")

	// Act
	left, cmd := pressed(t, opened, keyEnter)

	// Assert
	if !quits(cmd) || left.Destination().Dir != apiFeature {
		t.Errorf("enter on the feature worktree: quit %v, destination %q; want the program ended for %s",
			quits(cmd), left.Destination().Dir, apiFeature)
	}
}

func TestEnterOnAWorktreeThatIsGoneGoesNowhere(t *testing.T) {
	t.Parallel()

	// Arrange
	opened := typing(t, worktreesWorld().live(t, 120, 40), reposKey, "j", "j", "j")

	// Act
	stayed, cmd := pressed(t, opened, keyEnter)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("enter on a worktree gone: quit %v, destination %q; want to stay",
			quits(cmd), stayed.Destination().Dir)
	}

	requireScreen(t, stayed.View().Content, "~/src/api-gone is not there")
}

func TestWorktreesThatCannotBeReadSayWhyAndTheFavoritesStillList(t *testing.T) {
	t.Parallel()

	// Arrange
	working := reposWorld()
	working.dirs.worktreesErr = errWorktreesUnread

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	requireScreen(t, view, "Worktrees", "git worktree list failed", "Favorites", "~/src/web")
}

func TestAWorktreesBranchCannotDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	// A branch name is anyone's to write who can push to the repository.
	working := reposWorld()
	working.dirs.worktrees = []gitrepo.Worktree{
		{Dir: apiRoot, Branch: "main", Head: worktreeHead},
		{Dir: apiFeature, Branch: "feat/\x1b]0;owned\x07x", Head: worktreeHead},
	}

	// Act
	view := typing(t, working.live(t, 120, 40), reposKey).View().Content

	// Assert
	if strings.Contains(view, "\x1b]0;") {
		t.Errorf("a branch's escape reached the screen:\n%q", view)
	}
}
