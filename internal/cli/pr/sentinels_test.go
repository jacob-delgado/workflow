// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

// Each refusal pr and announce make answers errors.Is for the sentinel the
// package exports, in whatever words the forge has for it, so the root's exit
// families can name it.

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/pr"
)

func TestASecondPullRequestIsRefusedAsErrPullAlreadyOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	// Act
	_, err := clitest.Run(t, repo, "pr", "--yes")

	// Assert
	if !errors.Is(err, pr.ErrPullAlreadyOpen) {
		t.Errorf("pr = %v, want pr.ErrPullAlreadyOpen", err)
	}
}

func TestABranchWithNoCommitsIsRefusedAsErrNoCommitsToOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlab(t)
	repo := clitest.GitlabRepo(t, "fix/PROJ-2-thing")
	clitest.Git(t, repo, "reset", "--quiet", "--hard", "main")

	// Act
	_, err := clitest.Run(t, repo, "pr", "--yes")

	// Assert
	if !errors.Is(err, pr.ErrNoCommitsToOpen) {
		t.Errorf("pr = %v, want pr.ErrNoCommitsToOpen", err)
	}
}

func TestABranchWithNoPullRequestIsRefusedAsErrNoPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGh(t, clitest.GhResponses{Pulls: "[]"})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, clitest.ForgeCLIConfig)

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if !errors.Is(err, pr.ErrNoPullRequest) {
		t.Errorf("announce = %v, want pr.ErrNoPullRequest", err)
	}
}

func TestAWaitForTheCIOfAMergeIsRefusedAsErrOnlyReadyWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	var posts atomic.Int32

	clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.MergedPull("Add login")})
	repo := clitest.GithubRepo(t, "fix/PROJ-2-thing")
	clitest.WriteFile(t, repo, waitingConfig(webhook(t, &posts)))

	// Act
	_, err := clitest.Run(t, repo, "announce", "--when-green", "--yes")

	// Assert
	if !errors.Is(err, pr.ErrOnlyReadyWaits) {
		t.Errorf("announce --when-green = %v, want pr.ErrOnlyReadyWaits", err)
	}
}
