// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package pr_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// requireGitLabWords fails the test unless said names the merge request and
// never GitHub's noun or sigil.
func requireGitLabWords(t *testing.T, said, want string) {
	t.Helper()

	if !strings.Contains(said, want) || strings.Contains(said, "pull request") || strings.Contains(said, "#") {
		t.Errorf("said %q, want %q in GitLab's words, with no pull request or #", said, want)
	}
}

func TestPRRefusesASecondMergeRequestInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlabListing(t, clitest.OpenMergeRequest())
	repo := clitest.GitlabRepo(t, "fix/PROJ-2-thing")

	// Act
	_, err := clitest.Run(t, repo, "pr", "--yes")

	// Assert
	if err == nil {
		t.Fatal("pr = nil, want a second merge request refused")
	}

	requireGitLabWords(t, err.Error(), "an open merge request already exists for this branch")
	wantExit(t, err, 4)
}

func TestPRRefusesABranchWithNoCommitsInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlab(t)
	repo := clitest.GitlabRepo(t, "fix/PROJ-2-thing")
	clitest.Git(t, repo, "reset", "--quiet", "--hard", "main")

	// Act
	_, err := clitest.Run(t, repo, "pr", "--yes")

	// Assert
	if err == nil {
		t.Fatal("pr = nil, want a branch with no commits refused")
	}

	requireGitLabWords(t, err.Error(), "no branch with commits to open a merge request for")
	wantExit(t, err, 4)
}

func TestAnnounceRefusesABranchWithNoMergeRequestInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlab(t)
	repo := clitest.GitlabRepo(t, "fix/PROJ-2-thing")

	// Act
	_, err := clitest.Run(t, repo, "announce", "--yes")

	// Assert
	if err == nil {
		t.Fatal("announce = nil, want a branch with no merge request refused")
	}

	requireGitLabWords(t, err.Error(), "there is no merge request on this branch to announce")
	wantExit(t, err, 4)
}
