// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
	"time"
)

// gitlabForgeConfig points the forge at GitLab's CLI, with a webhook so
// announce gets past its messaging guard.
const gitlabForgeConfig = `{"forge":{"cli":true,"kind":"gitlab","host":"gitlab.com"},` +
	`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`

// gitlabRepo is prRepo on branch with a GitLab origin and gitlabForgeConfig.
func gitlabRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := prRepo(t, branch)
	git(t, repo, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	writeFile(t, repo, gitlabForgeConfig)

	return repo
}

// openMergeRequest is a listing holding merge request 7, open, as GitLab sends
// it to both the branch's lookup and the review queue.
func openMergeRequest() string {
	return `[{"iid":7,"web_url":"` + gitlabMergeRequest + `","title":"fix: token","state":"opened",` +
		`"created_at":"` + time.Now().Add(-50*time.Hour).UTC().Format(time.RFC3339) + `",` +
		`"author":{"username":"ben"},"references":{"full":"grp/proj!7"}}]`
}

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
	fakeGlabListing(t, openMergeRequest())
	repo := gitlabRepo(t, "fix/PROJ-2-thing")

	// Act
	_, err := run(t, repo, "pr", "--yes")

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
	fakeGlab(t)
	repo := gitlabRepo(t, "fix/PROJ-2-thing")
	git(t, repo, "reset", "--quiet", "--hard", "main")

	// Act
	_, err := run(t, repo, "pr", "--yes")

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
	fakeGlab(t)
	repo := gitlabRepo(t, "fix/PROJ-2-thing")

	// Act
	_, err := run(t, repo, "announce", "--yes")

	// Assert
	if err == nil {
		t.Fatal("announce = nil, want a branch with no merge request refused")
	}

	requireGitLabWords(t, err.Error(), "there is no merge request on this branch to announce")
	wantExit(t, err, 4)
}

func TestReviewsListsMergeRequestsInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGlabListing(t, openMergeRequest())
	repo := gitlabRepo(t, "work")

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "reviews")
	// Assert
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	if !strings.HasPrefix(printed.stdout, "!7  fix: token") || strings.Contains(printed.stdout, "#7") {
		t.Errorf("reviews printed %q, want each merge request marked with GitLab's sigil", printed.stdout)
	}
}

func TestReviewsWithNothingWaitingSaysSoInGitLabsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	fakeGlab(t)
	repo := gitlabRepo(t, "work")

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "reviews")
	// Assert
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	requireGitLabWords(t, printed.stderr, "No merge requests are waiting on your review.")
}
