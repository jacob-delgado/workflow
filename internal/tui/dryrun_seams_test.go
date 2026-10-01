// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// readOnly is the world's seams with every write left out, as a surface
// given no way to write has them.
func readOnly(w *world) tui.Deps {
	deps := w.deps()
	deps.Jira.Transition, deps.Jira.Comment, deps.Jira.Assign = nil, nil, nil
	deps.Jira.AddWorklog, deps.Jira.LinkPullRequest = nil, nil
	deps.Git.Stage, deps.Git.Unstage, deps.Git.CreateBranch, deps.Git.Checkout = nil, nil, nil, nil
	deps.Git.CreateWorktree, deps.Git.Finish, deps.Git.Commit, deps.Git.Push = nil, nil, nil, nil
	deps.Git.Amend, deps.Git.Fixup, deps.Git.Rebase = nil, nil, nil
	deps.Forge.CreatePullRequest, deps.Forge.EditPullRequest, deps.Forge.Rerun, deps.Forge.Merge = nil, nil, nil, nil
	deps.Messaging.Post = nil
	deps.Hooks = seams.Hooks{Run: nil, Existing: deps.Hooks.Existing, Write: nil}
	deps.Tasks = seams.Tasks{}

	return deps
}

func TestADryRunTurnsOnNoWriteThatIsNotThere(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	model := sized(t, tui.New(world.cfg, nil, readOnly(world)), 140, 40)

	// Act
	dry := model.WithDryRun()
	dry = drain(t, dry, dry.Init())

	// Assert

	for pane, unwanted := range map[string][]string{
		"2": {"push", "rebase"},
		"3": {"stage", "amend"},
		"4": {"re-run", "merge"},
		"5": {"announce"},
	} {
		refuseScreen(t, footerLine(typing(t, dry, pane).View().Content), unwanted...)
	}
}
