// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// The endpoints the review tests talk to.
const (
	githubSearchPath  = "/search/issues"
	gitlabReviewsPath = "/merge_requests"
	gitlabUserPath    = "/user"
)

// gitlabWhoami is the answer GitLab gives to /user for the review fixtures.
const gitlabWhoami = `{"username":"me"}`

// sameReview is whether two review requests match, comparing the timestamp with
// Equal rather than ==, which would also weigh the monotonic clock and location.
func sameReview(got, want forge.ReviewRequest) bool {
	sameTime := got.OpenedAt.Equal(want.OpenedAt)
	got.OpenedAt = want.OpenedAt

	return sameTime && got == want
}

func TestReviewRequestsListsWhatEachForgeReturns(t *testing.T) {
	t.Parallel()

	opened := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)

	cases := map[string]struct {
		kind   forge.Kind
		routes map[string]string
		asked  string
		want   forge.ReviewRequest
	}{
		"asks GitHub search": {
			kind: forge.KindGitHub,
			routes: map[string]string{
				githubSearchPath: `{"items":[{"number":42,"html_url":"https://github.com/ex/repo/pull/42",` +
					`"title":"fix: token","draft":false,"created_at":"2026-09-15T08:00:00Z",` +
					`"user":{"login":"ana"},"repository_url":"https://api.github.com/repos/ex/repo"}]}`,
			},
			asked: githubSearchPath,
			want: forge.ReviewRequest{
				Number: 42, URL: "https://github.com/ex/repo/pull/42", Title: "fix: token",
				Author: "ana", Repository: "ex/repo", CI: forge.CINone, OpenedAt: opened,
			},
		},
		"asks GitLab for its merge requests": {
			kind: forge.KindGitLab,
			routes: map[string]string{
				gitlabUserPath: gitlabWhoami,
				gitlabReviewsPath: `[{"iid":7,"web_url":"https://gitlab.com/grp/proj/-/merge_requests/7",` +
					`"title":"fix: token","draft":false,"created_at":"2026-09-15T08:00:00Z",` +
					`"author":{"username":"ben"},"references":{"full":"grp/proj!7"},` +
					`"head_pipeline":{"status":"success"}}]`,
			},
			asked: gitlabReviewsPath,
			want: forge.ReviewRequest{
				Number: 7, URL: "https://gitlab.com/grp/proj/-/merge_requests/7", Title: "fix: token",
				Author: "ben", Repository: "grp/proj", CI: forge.CIPassed, OpenedAt: opened,
			},
		},
		"GitLab with no pipeline yet reports no CI": {
			kind: forge.KindGitLab,
			routes: map[string]string{
				gitlabUserPath: gitlabWhoami,
				gitlabReviewsPath: `[{"iid":9,"web_url":"https://gitlab.com/grp/proj/-/merge_requests/9",` +
					`"title":"chore: bump","draft":false,"created_at":"2026-09-15T08:00:00Z",` +
					`"author":{"username":"cass"},"references":{"full":"grp/proj!9"}}]`,
			},
			asked: gitlabReviewsPath,
			want: forge.ReviewRequest{
				Number: 9, URL: "https://gitlab.com/grp/proj/-/merge_requests/9", Title: "chore: bump",
				Author: "cass", Repository: "grp/proj", CI: forge.CINone, OpenedAt: opened,
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeRouting(t, tt.routes)

			// Act
			reviews, err := client.ReviewRequests(t.Context(), tt.kind)

			// Assert
			if err != nil || len(reviews) != 1 {
				t.Fatalf("ReviewRequests = %+v, %v, want one review", reviews, err)
			}

			if got := reviews[0]; !sameReview(got, tt.want) {
				t.Errorf("ReviewRequests[0] = %+v, want %+v", got, tt.want)
			}

			if requestTo(*seen, tt.asked).method != http.MethodGet {
				t.Errorf("the reviews were not listed at %s; requests were %+v", tt.asked, *seen)
			}
		})
	}
}

func TestReviewRequestsFiltersGitLabByReviewer(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeRouting(t, map[string]string{gitlabUserPath: gitlabWhoami})

	// Act
	_, err := client.ReviewRequests(t.Context(), forge.KindGitLab)
	// Assert
	if err != nil {
		t.Fatalf("ReviewRequests: %v", err)
	}

	if query := requestTo(*seen, gitlabReviewsPath).query; !strings.Contains(query, "reviewer_username=me") ||
		!strings.Contains(query, "state=opened") {
		t.Errorf("the merge requests were not filtered by reviewer: %q", query)
	}
}

func TestReviewRequestsReportsAForgeFailure(t *testing.T) {
	t.Parallel()

	for name, kind := range map[string]forge.Kind{github: forge.KindGitHub, gitlab: forge.KindGitLab} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusInternalServerError, "")

			// Act
			_, err := client.ReviewRequests(t.Context(), kind)

			// Assert
			if err == nil {
				t.Errorf("ReviewRequests on %v returned no error for a failing forge", kind)
			}
		})
	}
}

func TestReviewRequestsReportsAGitLabListingFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitLab answers who you are, then sends something that is not a listing.
	client, _ := forgeRouting(t, map[string]string{
		gitlabUserPath:    gitlabWhoami,
		gitlabReviewsPath: `{"message":"nope"}`,
	})

	// Act
	_, err := client.ReviewRequests(t.Context(), forge.KindGitLab)

	// Assert
	if err == nil {
		t.Error("ReviewRequests returned no error for an unreadable listing")
	}
}

func TestReviewRequestsForAnUnknownForge(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _ := forgeAnswering(t, http.StatusOK, `{}`)

	// Act
	_, err := client.ReviewRequests(t.Context(), forge.KindUnknown)

	// Assert
	if err == nil {
		t.Error("ReviewRequests for an unknown forge returned no error")
	}
}

func TestOldestFirstPutsTheLongestWaitingFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// Fourteen requests opened at two moments, turn about: more than a sort
	// orders by insertion, so an unstable sort would reorder the ties.
	const requestCount = 14

	monday := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	moments := [2]time.Time{monday.Add(48 * time.Hour), monday}

	answered := make([]forge.ReviewRequest, 0, requestCount)
	for i := range requestCount {
		answered = append(answered, forge.ReviewRequest{Number: requestCount - i, OpenedAt: moments[i%2]})
	}

	// Act
	queue := forge.OldestFirst(answered)

	// Assert
	got := make([]int, 0, len(queue))
	for _, request := range queue {
		got = append(got, request.Number)
	}

	// Monday's requests first, and each moment's in the order the forge gave.
	if want := []int{13, 11, 9, 7, 5, 3, 1, 14, 12, 10, 8, 6, 4, 2}; !slices.Equal(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}

	if answered[0].Number != requestCount {
		t.Errorf("the forge's answer was reordered in place: %v", answered)
	}
}
