// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The branch listing behind the switcher and the Issues list's marks: which
// branches name an issue, by their name or a link made by hand, and which
// another worktree has checked out.

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// spikeBranch names no issue: only a link made by hand ties it to one.
const spikeBranch = "spike-retries"

// liveWithLinks is repo's interface with links as the branches linked to an
// issue by hand, which the world otherwise leaves unread.
func liveWithLinks(t *testing.T, repo *world, links map[string]string) tui.Model {
	t.Helper()

	deps := repo.deps()
	deps.Git.IssueLinks = func(context.Context) map[string]string { return links }
	model := sized(t, tui.New(repo.cfg, nil, deps), 160, 40)

	return drain(t, model, model.Init())
}

func TestTheSwitcherOffersABranchLinkedByHand(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := cleanSwitcher()
	repo.branches = []string{featureName, spikeBranch}
	started := liveWithLinks(t, repo, map[string]string{spikeBranch: "PROJ-388"})

	// Act
	view := typing(t, started, "2", "s").View().Content

	// Assert
	requireScreen(t, view, "PROJ-388", spikeBranch)
}

func TestTheSwitcherIgnoresAWorktreeOnNoBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	// The other worktree's HEAD is detached, so it holds no branch.
	repo := retriesInAWorktree()
	repo.dirs.worktrees[1].Branch = ""

	// Act
	view := typing(t, repo.live(t, 160, 40), "2", "s").View().Content

	// Assert
	requireScreen(t, view, otherTaskBranch)
	refuseScreen(t, view, "(worktree at")
}
