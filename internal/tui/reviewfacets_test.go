// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

const (
	// filterKey opens the Reviews pane's filter.
	filterKey = "f"
	// exampleRepo, authorKwan and authorMira are where the queued requests are
	// and who asks for them.
	exampleRepo = "example/repo"
	authorKwan  = "kwan"
	authorMira  = "mira"
)

// facetsWorld queues four requests that differ in every facet: #5, a draft by
// kwan in example/repo with CI running, waiting longest; #12 by kwan in
// example/other, passed; #3 by mira in no repository, with no CI; and #7 by
// mira in example/repo, failed, the newest.
func facetsWorld() *world {
	repo := newWorld()
	repo.reviews = []forge.ReviewRequest{
		{
			Number: 7, URL: reviewNewerURL, Title: "fix flaky redaction test", Author: authorMira,
			Repository: exampleRepo, CI: forge.CIFailed, OpenedAt: testNow().Add(-3 * time.Hour),
		},
		{
			Number: 12, URL: reviewOlderURL, Title: "add request retries", Author: authorKwan,
			Repository: "example/other", CI: forge.CIPassed, OpenedAt: testNow().Add(-26 * time.Hour),
		},
		{
			Number: 5, URL: "https://github.com/example/repo/pull/5", Title: "drop the old retry flag",
			Author: authorKwan, Repository: exampleRepo, Draft: true, CI: forge.CIRunning,
			OpenedAt: testNow().Add(-40 * time.Hour),
		},
		{
			Number: 3, URL: "https://example.com/pull/3", Title: "a request with no repository",
			Author: authorMira, CI: forge.CINone, OpenedAt: testNow().Add(-10 * time.Hour),
		},
	}

	return repo
}

// requestNumber finds a queued request's number at the start of its row, which
// ends in who asks for the review.
var requestNumber = regexp.MustCompile(`#(\d+) [^\n]*by (?:kwan|mira)`)

// listedRequests is the numbers of the requests the Reviews pane lists, in
// order.
func listedRequests(view string) []string {
	matches := requestNumber.FindAllStringSubmatch(plain(view), -1)
	listed := make([]string, 0, len(matches))

	for _, match := range matches {
		listed = append(listed, match[1])
	}

	return listed
}

// filtering is the Reviews pane after f, the keys given, and enter.
func filtering(t *testing.T, repo *world, keys ...string) string {
	t.Helper()

	typed := slices.Concat([]string{"6", filterKey}, keys, []string{keyEnter})

	return typing(t, repo.live(t, 120, 40), typed...).View().Content
}

func TestFOpensTheReviewsFilterWithCounts(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, facetsWorld().live(t, 120, 40), "6", filterKey).View().Content

	// Assert
	requireScreen(t, view, "Filter", "no repository  1", "example/other  1", "example/repo  2",
		"CI failed  1", "CI passed  1", "CI running  1", "CI none  1", "draft  1", "ready  3",
		"by kwan  2", "by mira  2")
}

func TestFacetsWidenWithinAndNarrowTogether(t *testing.T) {
	t.Parallel()

	// The filter lists no repository, example/other, example/repo, CI failed,
	// CI passed, CI running, CI none, draft, ready, by kwan, by mira.
	cases := map[string]struct {
		keys []string
		want []string
	}{
		"two repositories list either": {
			keys: []string{downAction, keySpace, downAction, keySpace},
			want: []string{"5", "12", "7"},
		},
		"a repository and a CI state list both": {
			keys: []string{downAction, downAction, keySpace, downAction, keySpace},
			want: []string{"7"},
		},
		"two CI states list either": {
			keys: slices.Concat(steps(3), []string{keySpace}, steps(3), []string{keySpace}),
			want: []string{"3", "7"},
		},
		"draft and an author list both": {
			keys: slices.Concat(steps(7), []string{keySpace}, steps(2), []string{keySpace}),
			want: []string{"5"},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			view := filtering(t, facetsWorld(), tt.keys...)

			// Assert
			if got := listedRequests(view); !slices.Equal(got, tt.want) {
				t.Errorf("listed %v, want %v:\n%s", got, tt.want, plain(view))
			}
		})
	}
}

func TestTheLineAboveTheQueueNamesTheSortAndTheFilters(t *testing.T) {
	t.Parallel()

	// Act
	view := filtering(t, facetsWorld(), downAction, downAction, keySpace, downAction, keySpace)

	// Assert
	requireScreen(t, view, "oldest first · filters: example/repo, CI failed")
}

func TestAFilterNothingMatchesSaysSo(t *testing.T) {
	t.Parallel()

	// Act
	// draft and CI failed: #5 is the draft, and its CI is running.
	view := filtering(t, facetsWorld(), slices.Concat(steps(3), []string{keySpace}, steps(4),
		[]string{keySpace})...)

	// Assert
	requireScreen(t, view, "No review request matches the filters.")
}

func TestAPickedValueNoRequestHoldsIsStillOfferedAtZero(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := facetsWorld()
	filtered := typing(t, repo.live(t, 120, 40),
		slices.Concat([]string{"6", filterKey}, steps(1), []string{keySpace, keyEnter})...)
	repo.reviews = repo.reviews[:1]
	refreshed := typing(t, filtered, "r")

	// Act
	view := typing(t, refreshed, filterKey).View().Content

	// Assert
	requireScreen(t, view, "example/other  0")
}

func TestEscapeLeavesTheQueueUnfiltered(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, facetsWorld().live(t, 120, 40), "6", filterKey, downAction, keySpace, keyEsc).View().Content

	// Assert
	if got := listedRequests(view); len(got) != 4 {
		t.Errorf("listed %v, want all four requests:\n%s", got, plain(view))
	}
}

func TestTheFilterHoldsAcrossARefresh(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := facetsWorld()
	filtered := typing(t, repo.live(t, 120, 40), "6", filterKey, downAction, keySpace, keyEnter)

	// Act
	view := typing(t, filtered, "r").View().Content

	// Assert
	if got := listedRequests(view); !slices.Equal(got, []string{"12"}) {
		t.Errorf("listed %v, want only example/other's #12:\n%s", got, plain(view))
	}
}

func TestTheFilterHoldsAcrossASort(t *testing.T) {
	t.Parallel()

	// Arrange
	filtered := typing(t, facetsWorld().live(t, 120, 40),
		slices.Concat([]string{"6", filterKey}, steps(9), []string{keySpace, keyEnter})...)

	// Act
	view := typing(t, filtered, "s").View().Content

	// Assert
	if got := listedRequests(view); !slices.Equal(got, []string{"12", "5"}) {
		t.Errorf("listed %v, want kwan's two, newest first:\n%s", got, plain(view))
	}

	requireScreen(t, view, "newest first · filters: by kwan")
}

// steps is the down key, count times.
func steps(count int) []string {
	return slices.Repeat([]string{downAction}, count)
}
