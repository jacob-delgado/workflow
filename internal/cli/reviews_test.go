// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// reviewsForgeConfig points the forge at its CLI, so the reviews listing reads
// through the fake gh rather than a live GitHub.
const reviewsForgeConfig = `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`

// reviewItem is one pull request as GitHub's search endpoint sends it: opened at
// created, by author, in repository (empty for one the search does not name).
func reviewItem(number int, author, repository string, created time.Time) string {
	repositoryURL := ""
	if repository != "" {
		repositoryURL = "https://api.github.com/repos/" + repository
	}

	return `{"number":` + strconv.Itoa(number) + `,"html_url":"https://github.com/ex/repo/pull/` +
		strconv.Itoa(number) + `","title":"fix: token","draft":false,"created_at":"` +
		created.UTC().Format(time.RFC3339) + `","user":{"login":"` + author +
		`"},"repository_url":"` + repositoryURL + `"}`
}

// reviewSearch wraps items as the search endpoint's {"items":[...]} answer.
func reviewSearch(items ...string) string {
	return `{"items":[` + strings.Join(items, ",") + `]}`
}

// reviewsRepo is a repository whose remote points at GitHub, so the reviews
// command has a forge to resolve, with the fake gh answering the search.
func reviewsRepo(t *testing.T) string {
	t.Helper()

	repo := githubRepo(t, "work")
	writeFile(t, repo, reviewsForgeConfig)

	return repo
}

func TestReviewsWithoutAForgeReportsSo(t *testing.T) {
	// Act
	// No repository, so no remote names a forge.
	_, err := run(t, t.TempDir(), "reviews")

	// Assert
	if !errors.Is(err, forge.ErrNotARemote) {
		t.Errorf("reviews without a forge to ask = %v, want it to say there is no remote", err)
	}

	wantExit(t, err, 3)
}

func TestReviewsOnAHostThatNamesNoForgeReportsSo(t *testing.T) {
	// Arrange
	// The hostname says neither GitHub nor GitLab, and no forge.kind says which.
	repo := repoWithRemote(t, "git@git.example.com:acme/thing.git")

	// Act
	_, err := run(t, repo, "reviews")

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) {
		t.Errorf("reviews on a host that names no forge = %v, want it to say it cannot tell the forge", err)
	}

	wantExit(t, err, 3)
}

func TestReviewsShowsTheAuthorRepositoryCIAndAge(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{search: reviewSearch(
		reviewItem(7, "ben", "grp/proj", time.Now().Add(-50*time.Hour)),
	)})
	repo := reviewsRepo(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "reviews")
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	// Assert
	// GitHub's search carries no CI, so the forge reports it as none rather than
	// paying a request per entry to learn it.
	wants := []string{"fix: token", "(grp/proj)", "by ben", "CI none", "2d", "https://github.com/ex/repo/pull/7"}
	for _, want := range wants {
		if !strings.Contains(printed.stdout, want) {
			t.Errorf("the review line on stdout is missing %q:\n%s", want, printed.stdout)
		}
	}

	if printed.stderr != "" {
		t.Errorf("reviews said something beside its lines:\n%s", printed.stderr)
	}
}

func TestReviewsOmitsAnUnknownRepository(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{search: reviewSearch(
		reviewItem(5, "ana", "", time.Now().Add(-time.Hour)),
	)})
	repo := reviewsRepo(t)

	// Act
	output, err := run(t, repo, "reviews")
	if err != nil {
		t.Fatalf("reviews: %v (%s)", err, output)
	}

	// Assert
	if strings.Contains(output, "()") || !strings.Contains(output, "by ana") {
		t.Errorf("a review with no repository did not render cleanly:\n%s", output)
	}
}

func TestReviewsWithNothingWaitingSaysSo(t *testing.T) {
	// Arrange
	// The default search answers with no items.
	fakeGh(t, ghResponses{})
	repo := reviewsRepo(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "reviews")
	if err != nil {
		t.Fatalf("reviews: %v (%+v)", err, printed)
	}

	// Assert
	// An empty queue has no lines to read; saying so is commentary, so a script
	// counting the lines on stdout counts none.
	if !strings.Contains(printed.stderr, "No pull requests are waiting on your review.") || printed.stdout != "" {
		t.Errorf("an empty queue was not reported on stderr alone:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestReviewsReportsAForgeFailure(t *testing.T) {
	// Arrange
	// An unreadable search body makes the forge client fail to decode the answer.
	fakeGh(t, ghResponses{search: "{"})
	repo := reviewsRepo(t)

	// Act
	_, err := run(t, repo, "reviews")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "review requests") {
		t.Errorf("reviews returned %v, want the read failure", err)
	}
}

func TestReviewsAsJSONReportsEachOldestFirstWithItsAge(t *testing.T) {
	// Arrange
	now := time.Now()
	fakeGh(t, ghResponses{search: reviewSearch(
		reviewItem(30, "ana", "ex/repo", now.Add(-30*time.Minute)),
		reviewItem(10, "ben", "ex/repo", now.Add(-74*time.Hour)),
		reviewItem(20, "cass", "ex/repo", now.Add(-5*time.Hour)),
		// Opened "in the future", which humanizeAge clamps to no age at all.
		reviewItem(40, "dee", "ex/repo", now.Add(time.Hour)),
	)})
	repo := reviewsRepo(t)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "reviews", "--json")
	if err != nil {
		t.Fatalf("reviews --json: %v (%+v)", err, printed)
	}

	// Assert
	var reports []struct {
		Number int    `json:"number"`
		CI     string `json:"ci"`
		Age    string `json:"age"`
	}

	output := printed.stdout

	err = json.Unmarshal([]byte(output), &reports)
	if err != nil || printed.stderr != "" {
		t.Fatalf("stdout is not the JSON alone: %v\nstdout:\n%s\nstderr:\n%s", err, output, printed.stderr)
	}

	if len(reports) != 4 || reports[0].Number != 10 || reports[0].CI != "none" {
		t.Fatalf("reviews JSON = %+v, want four reviews oldest-first with CI none", reports)
	}

	ageByNumber := make(map[int]string, len(reports))
	for _, report := range reports {
		ageByNumber[report.Number] = report.Age
	}

	for number, want := range map[int]string{10: "3d", 20: "5h", 30: "30m", 40: "0m"} {
		if got := ageByNumber[number]; got != want {
			t.Errorf("review #%d age = %q, want %q\n%s", number, got, want, output)
		}
	}
}
