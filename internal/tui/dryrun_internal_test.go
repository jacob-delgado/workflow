// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// Dry run holds writes back at the seam, not only at each call site's guard.
// This reaches model.deps to call every write seam directly, which is what the
// "fails safe" invariant is about, so it lives in the tui package.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
)

func TestDryRunHoldsBackEveryWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached []string

	//nolint:unparam // the recorder always succeeds; its nil error is the fakes' return.
	note := func(name string) error {
		reached = append(reached, name)

		return nil
	}
	deps := Deps{
		Jira: JiraDeps{
			Transition:      func(jira.Key, jira.Transition, []jira.FieldValue) error { return note("Transition") },
			Comment:         func(jira.Key, string) (jira.Comment, error) { return jira.Comment{}, note("Comment") },
			LinkPullRequest: func(jira.Key, string, string) error { return note("LinkPullRequest") },
		},
		Git: GitDeps{
			Stage:          func(gitrepo.Change) error { return note("Stage") },
			Unstage:        func(gitrepo.Change) error { return note("Unstage") },
			CreateBranch:   func(string, string) error { return note("CreateBranch") },
			Checkout:       func(string) error { return note("Checkout") },
			CreateWorktree: func(string, string) (string, error) { return "", note("CreateWorktree") },
			Commit:         func(string) (proc.Output, error) { return proc.Output{}, note("Commit") },
			Push:           func(string) (proc.Output, error) { return proc.Output{}, note("Push") },
			Rebase:         func(string) (proc.Output, error) { return proc.Output{}, note("Rebase") },
		},
		Forge: ForgeDeps{
			CreatePullRequest: func(forge.NewPullRequest) (forge.PullRequest, error) {
				return forge.PullRequest{}, note("CreatePullRequest")
			},
		},
		Slack: SlackDeps{Post: func(string, string) error { return note("Post") }},
		Hooks: HookDeps{
			Run:   func(string) (proc.Output, error) { return proc.Output{}, note("Run") },
			Write: func(hooks.Generated) error { return note("Write") },
		},
	}

	held := New(config.Config{}, nil, deps).WithDryRun().deps

	// Act
	_ = held.Jira.Transition("K", jira.Transition{}, nil)
	_, _ = held.Jira.Comment("K", "t")
	_ = held.Jira.LinkPullRequest("K", "u", "t")
	_ = held.Git.Stage(gitrepo.Change{})
	_ = held.Git.Unstage(gitrepo.Change{})
	_ = held.Git.CreateBranch("b", "s")
	_ = held.Git.Checkout("b")
	_, _ = held.Git.CreateWorktree("b", "s")
	_, _ = held.Git.Commit("m")
	_, _ = held.Git.Push("b")
	_, _ = held.Git.Rebase("base")
	_, _ = held.Forge.CreatePullRequest(forge.NewPullRequest{})
	_ = held.Slack.Post("c", "t")
	_, _ = held.Hooks.Run("pre-commit")
	_ = held.Hooks.Write(hooks.Generated{})

	// Assert
	if len(reached) != 0 {
		t.Errorf("dry run let writes reach the fake: %v", reached)
	}
}

func TestHeldBackLeavesAnUnavailableSeamNil(t *testing.T) {
	t.Parallel()

	// Act
	// A nil seam means the feature is unavailable; dry run must not turn it on.
	held := heldBack(Deps{})

	// Assert
	if held.Slack.Post != nil || held.Git.Push != nil || held.Jira.Comment != nil ||
		held.Forge.CreatePullRequest != nil || held.Hooks.Write != nil {
		t.Error("heldBack made an unavailable write seam callable")
	}
}
