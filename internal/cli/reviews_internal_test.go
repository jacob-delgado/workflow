// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

// runReviews reads from a forge a black-box test cannot point at a fake, so
// these drive it directly with fake seams. errServiceDown is shared with the
// status seam tests.

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// reviewsNow is the fixed clock the review fixtures are aged against.
func reviewsNow() time.Time {
	return time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
}

// reviewsFixture is two reviews waiting: an older one whose CI passed and a
// newer one whose CI is still running.
func reviewsFixture() reviewsSeams {
	now := reviewsNow()

	return reviewsSeams{
		Now: reviewsNow,
		List: func() ([]forge.ReviewRequest, error) {
			return []forge.ReviewRequest{
				{
					Number: 3, URL: "https://forge/pull/3", Title: "add: retries", Author: "ana",
					Repository: "ex/repo", CI: forge.CIRunning, OpenedAt: now.Add(-2 * time.Hour),
				},
				{
					Number: 7, URL: "https://forge/mr/7", Title: "fix: token", Author: "ben",
					Repository: "grp/proj", CI: forge.CIPassed, OpenedAt: now.Add(-50 * time.Hour),
				},
			}, nil
		},
	}
}

func TestReviewsListsTheLongestWaitingFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runReviews(&out, reviewsFixture(), false)
	if err != nil {
		t.Fatalf("runReviews: %v", err)
	}

	// Assert
	if seven, three := strings.Index(out.String(), "#7"), strings.Index(out.String(), "#3"); seven > three {
		t.Errorf("the longest-waiting review is not first:\n%s", out.String())
	}
}

func TestReviewsShowsTheAuthorRepositoryCIAndAge(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runReviews(&out, reviewsFixture(), false)
	if err != nil {
		t.Fatalf("runReviews: %v", err)
	}

	// Assert
	line := out.String()
	for _, want := range []string{"fix: token", "(grp/proj)", "by ben", "CI passed", "2d", "https://forge/mr/7"} {
		if !strings.Contains(line, want) {
			t.Errorf("the review line is missing %q:\n%s", want, line)
		}
	}
}

func TestReviewsOmitsAnUnknownRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := reviewsFixture()
	seams.List = func() ([]forge.ReviewRequest, error) {
		return []forge.ReviewRequest{
			{Number: 5, URL: "https://forge/pr/5", Title: "fix: it", Author: "ana", OpenedAt: reviewsNow()},
		}, nil
	}

	var out bytes.Buffer

	// Act
	err := runReviews(&out, seams, false)
	if err != nil {
		t.Fatalf("runReviews: %v", err)
	}

	// Assert
	if line := out.String(); strings.Contains(line, "()") || !strings.Contains(line, "by ana") {
		t.Errorf("a review with no repository did not render cleanly:\n%s", line)
	}
}

func TestReviewsWithNothingWaitingSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := reviewsFixture()
	seams.List = func() ([]forge.ReviewRequest, error) { return nil, nil }

	var out bytes.Buffer

	// Act
	err := runReviews(&out, seams, false)
	if err != nil {
		t.Fatalf("runReviews: %v", err)
	}

	// Assert
	if !strings.Contains(out.String(), "No pull requests are waiting on your review.") {
		t.Errorf("an empty queue was not reported:\n%s", out.String())
	}
}

func TestReviewsReportsAForgeFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	seams := reviewsFixture()
	seams.List = func() ([]forge.ReviewRequest, error) { return nil, errServiceDown }

	var out bytes.Buffer

	// Act
	err := runReviews(&out, seams, false)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "review requests") {
		t.Errorf("runReviews returned %v, want the read failure", err)
	}
}

func TestReviewsAsJSONReportsEachReview(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := runReviews(&out, reviewsFixture(), true)
	if err != nil {
		t.Fatalf("runReviews: %v", err)
	}

	// Assert
	var reports []struct {
		Number int    `json:"number"`
		Author string `json:"author"`
		CI     string `json:"ci"`
		Age    string `json:"age"`
	}

	err = json.Unmarshal(out.Bytes(), &reports)
	if err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}

	if len(reports) != 2 || reports[0].Number != 7 || reports[0].CI != "passed" || reports[0].Age != "2d" {
		t.Errorf("reviews JSON = %+v, want the oldest first with its CI and age", reports)
	}
}

func TestHumanizeAgeCountsMinutesHoursThenDays(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ago  time.Duration
		want string
	}{
		"minutes":     {ago: 30 * time.Minute, want: "30m"},
		"hours":       {ago: 5 * time.Hour, want: "5h"},
		"days":        {ago: 74 * time.Hour, want: "3d"},
		"in the past": {ago: -time.Hour, want: "0m"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := humanizeAge(reviewsNow(), reviewsNow().Add(-tt.ago))

			// Assert
			if got != tt.want {
				t.Errorf("humanizeAge(-%s) = %q, want %q", tt.ago, got, tt.want)
			}
		})
	}
}
