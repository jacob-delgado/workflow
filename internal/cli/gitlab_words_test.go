// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

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

func TestReviewsListsMergeRequestsInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlabListing(t, clitest.OpenMergeRequest())
	repo := clitest.GitlabRepo(t, "work")

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "reviews")
	// Assert
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	if !strings.HasPrefix(printed.Stdout, "!7  fix: token") || strings.Contains(printed.Stdout, "#7") {
		t.Errorf("reviews printed %q, want each merge request marked with GitLab's sigil", printed.Stdout)
	}
}

func TestReviewsWithNothingWaitingSaysSoInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeGlab(t)
	repo := clitest.GitlabRepo(t, "work")

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "reviews")
	// Assert
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	requireGitLabWords(t, printed.Stderr, "No merge requests are waiting on your review.")
}
