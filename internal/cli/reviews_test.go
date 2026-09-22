// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
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
	_, err := run(t, t.TempDir(), "reviews")

	// Assert
	if err == nil {
		t.Error("reviews without a forge to ask returned no error")
	}
}

func TestReviewsShowsTheAuthorRepositoryCIAndAge(t *testing.T) {
	// Arrange
	fakeGh(t, ghResponses{search: reviewSearch(
		reviewItem(7, "ben", "grp/proj", time.Now().Add(-50*time.Hour)),
	)})
	repo := reviewsRepo(t)

	// Act
	output, err := run(t, repo, "reviews")
	if err != nil {
		t.Fatalf("reviews: %v (%s)", err, output)
	}

	// Assert
	// GitHub's search carries no CI, so the forge reports it as none rather than
	// paying a request per entry to learn it.
	wants := []string{"fix: token", "(grp/proj)", "by ben", "CI none", "2d", "https://github.com/ex/repo/pull/7"}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("the review line is missing %q:\n%s", want, output)
		}
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
	output, err := run(t, repo, "reviews")
	if err != nil {
		t.Fatalf("reviews: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "No pull requests are waiting on your review.") {
		t.Errorf("an empty queue was not reported:\n%s", output)
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
	output, err := run(t, repo, "reviews", "--json")
	if err != nil {
		t.Fatalf("reviews --json: %v (%s)", err, output)
	}

	// Assert
	var reports []struct {
		Number int    `json:"number"`
		CI     string `json:"ci"`
		Age    string `json:"age"`
	}

	err = json.Unmarshal([]byte(output), &reports)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, output)
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
