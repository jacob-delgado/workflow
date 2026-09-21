// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// manyReviews is a queue of n requests, request i opened i hours ago — so the
// oldest-first list runs from request n down to request 1 — each with a title
// and URL a test can match without one being a substring of another.
func manyReviews(n int) []forge.ReviewRequest {
	out := make([]forge.ReviewRequest, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, forge.ReviewRequest{
			Number: i, URL: "https://example.com/pr/" + strconv.Itoa(i),
			Title: "queued change (" + strconv.Itoa(i) + ")", Author: "dev",
			Repository: "example/repo", CI: forge.CINone,
			OpenedAt: testNow().Add(-time.Duration(i) * time.Hour),
		})
	}

	return out
}

// repeatKey is a key pressed n times.
func repeatKey(key string, n int) []string {
	keys := make([]string, n)
	for index := range keys {
		keys[index] = key
	}

	return keys
}

// topReviewNumber reads the request number on the detail's first visible row of
// a manyReviews queue, so a scrolled test can name the row without knowing the
// scroll offset.
func topReviewNumber(t *testing.T, view string) int {
	t.Helper()

	for line := range strings.SplitSeq(plain(view), "\n") {
		_, after, found := strings.Cut(line, "queued change (")
		if !found {
			continue
		}

		inside, _, ok := strings.Cut(after, ")")
		if !ok {
			continue
		}

		number, err := strconv.Atoi(inside)
		if err == nil {
			return number
		}
	}

	t.Fatalf("no review row on screen:\n%s", plain(view))

	return 0
}

// The pull requests the review queue lists, oldest last in this literal so a test
// can prove the pane reorders them oldest-first.
const (
	reviewNewerURL = "https://github.com/example/repo/pull/7"
	reviewOlderURL = "https://github.com/example/other/pull/12"
)

// withReviews is a world whose forge has two pull requests waiting on the user's
// review, opened three hours and just over a day ago.
func withReviews() *world {
	repo := newWorld()
	repo.reviews = []forge.ReviewRequest{
		{
			Number: 7, URL: reviewNewerURL, Title: "fix flaky redaction test", Author: "mira",
			Repository: "example/repo", CI: forge.CIFailed, OpenedAt: testNow().Add(-3 * time.Hour),
		},
		{
			Number: 12, URL: reviewOlderURL, Title: "add request retries", Author: "kwan",
			Repository: "example/other", CI: forge.CIPassed, OpenedAt: testNow().Add(-26 * time.Hour),
		},
	}

	return repo
}

func TestTheReviewsPaneCountsThePullRequestsWaiting(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	view := repo.live(t, 120, 40).View().Content

	// Assert
	requireScreen(t, view, "2 review requests waiting")
}

func TestTheReviewsPaneListsEachRequestOldestFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	view := plain(typing(t, repo.live(t, 120, 40), "6").View().Content)

	// Assert
	requireScreen(t, view, "#12 add request retries", "by kwan", "1d ago",
		"#7 fix flaky redaction test", "by mira", "3h ago", "example/other")

	// The queue is worked oldest-first, so the request waiting longest is listed
	// above the newer one.
	if older, newer := strings.Index(view, "#12"), strings.Index(view, "#7"); older > newer {
		t.Errorf("the oldest request is listed below the newer one:\n%s", view)
	}
}

func TestOpeningAReviewOpensItsPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "o")

	// Assert
	// The oldest request is selected first, so it is the one that opens.
	if got := repo.asked("browse " + reviewOlderURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the oldest request", got)
	}
}

func TestCopyingAReviewCopiesItsURL(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	view := typing(t, repo.live(t, 120, 40), "6", "y").View().Content

	// Assert
	if got := repo.asked("copy " + reviewOlderURL); len(got) != 1 {
		t.Errorf("copy calls = %v, want one for the oldest request", got)
	}

	requireScreen(t, view, "copied "+reviewOlderURL)
}

func TestMovingDownTheReviewsSelectsTheNext(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "down", "o")

	// Assert
	if got := repo.asked("browse " + reviewNewerURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the request below the first", got)
	}
}

func TestMovingBackUpTheReviewsReselectsTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "down", "up", "o")

	// Assert
	if got := repo.asked("browse " + reviewOlderURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the first request after moving back up", got)
	}
}

func TestClickingAReviewSelectsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	// Focused, the queue is drawn in the detail: the first request on its top row
	// and the second on the row below.
	repo := withReviews()
	model := typing(t, repo.live(t, 120, 40), "6")

	// Act
	typing(t, click(t, model, 40, 3), "o")

	// Assert
	if got := repo.asked("browse " + reviewNewerURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the clicked request", got)
	}
}

func TestClickingBelowTheReviewsKeepsTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// The two requests occupy the detail's first rows; row 8 is the empty space
	// below them. Select the second request first, off the default first row, so a
	// click that wrongly reset or dropped the selection would be visible.
	repo := withReviews()
	model := typing(t, repo.live(t, 120, 40), "6", "down")

	// Act
	typing(t, click(t, model, 40, 8), "o")

	// Assert
	// A click on nothing leaves the second request selected, so o opens it rather
	// than the default first row.
	if got := repo.asked("browse " + reviewNewerURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want the off-default selection kept by a click on empty space", got)
	}
}

func TestAReviewWithoutARepositoryStillListsItsAuthor(t *testing.T) {
	t.Parallel()

	// Arrange
	// A forge that does not report the repository — the queue still names who
	// wants the review.
	repo := newWorld()
	repo.reviews = []forge.ReviewRequest{{
		Number: 3, URL: "https://example.com/3", Title: "tidy imports", Author: "solo",
		OpenedAt: testNow().Add(-time.Hour),
	}}

	// Act
	view := typing(t, repo.live(t, 120, 40), "6").View().Content

	// Assert
	requireScreen(t, view, "#3 tidy imports", "by solo")
}

func TestRefreshingKeepsTheSelectedReviewWhenTheQueueReorders(t *testing.T) {
	t.Parallel()

	// Arrange
	// Select the second request, then let the queue come back with an even older
	// request prepended, so every row shifts down.
	repo := withReviews()
	model := typing(t, repo.live(t, 120, 40), "6", "down")
	repo.reviews = append([]forge.ReviewRequest{{
		Number: 99, URL: "https://example.com/99", Title: "old migration", Author: "wren",
		Repository: "example/legacy", CI: forge.CINone, OpenedAt: testNow().Add(-72 * time.Hour),
	}}, repo.reviews...)

	// Act
	typing(t, model, "r", "o")

	// Assert
	// The cursor followed its pull request by URL across the reorder, so o opens
	// the one that was selected rather than whatever now sits at its old row.
	if got := repo.asked("browse " + reviewNewerURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want the originally-selected request after a reorder", got)
	}
}

func TestALongReviewQueueScrollsToFollowTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// Twenty requests overflow the detail at this height, so moving to the newest
	// (the bottom of the oldest-first list) must scroll.
	repo := newWorld()
	repo.reviews = manyReviews(20)

	keys := append([]string{"6"}, repeatKey("down", 19)...)

	// Act
	view := typing(t, repo.live(t, 120, 16), keys...).View().Content

	// Assert
	// The selected newest request is on screen and the oldest has scrolled off.
	requireScreen(t, view, "queued change (1)")
	refuseScreen(t, plain(view), "queued change (20)")
}

func TestClickingAScrolledReviewSelectsTheRowInView(t *testing.T) {
	t.Parallel()

	// Arrange
	// Scroll the queue to the bottom, so the detail's top row is no longer the
	// first request; the request now there is the one a top-row click must select.
	repo := newWorld()
	repo.reviews = manyReviews(20)

	keys := append([]string{"6"}, repeatKey("down", 19)...)
	scrolled := typing(t, repo.live(t, 120, 16), keys...)

	top := topReviewNumber(t, scrolled.View().Content)
	if top == 20 {
		t.Fatalf("the queue did not scroll; the first request is still on the top row")
	}

	// Act
	typing(t, click(t, scrolled, 40, 2), "o")

	// Assert
	// The top-row click opens the request scrolled into view, which holds only if
	// the click adds the scroll offset rather than reading the first request.
	if got := repo.asked("browse https://example.com/pr/" + strconv.Itoa(top)); len(got) != 1 {
		t.Errorf("browse calls = %v, want the scrolled-in top request #%d", got, top)
	}
}

func TestClickingAfterTheQueueShrinksMapsToTheVisibleRow(t *testing.T) {
	t.Parallel()

	// Arrange
	// Scroll a long queue to the bottom, then refresh to a much shorter one. Unless
	// the scroll offset is re-clamped to the new length, a click maps past the end
	// of the shorter list.
	repo := newWorld()
	repo.reviews = manyReviews(20)

	keys := append([]string{"6"}, repeatKey("down", 19)...)
	scrolled := typing(t, repo.live(t, 120, 16), keys...)

	repo.reviews = manyReviews(3)
	afterRefresh := typing(t, scrolled, "r")
	top := topReviewNumber(t, afterRefresh.View().Content)

	// Act
	typing(t, click(t, afterRefresh, 40, 2), "o")

	// Assert
	// The top-row click opens the request now on that row, not one stranded by a
	// scroll offset left over from the longer queue.
	if got := repo.asked("browse https://example.com/pr/" + strconv.Itoa(top)); len(got) != 1 {
		t.Errorf("browse calls = %v, want the visible top request #%d after the queue shrank", got, top)
	}
}

func TestRefreshingTheReviewsReloadsTheQueue(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withReviews()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "r")

	// Assert
	// Once at startup, once for the refresh.
	if got := repo.asked("reviews"); len(got) != 2 {
		t.Errorf("reviews calls = %d, want two: the initial load and the refresh", len(got))
	}
}

func TestAnEmptyReviewQueueSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// newWorld's forge has no review requests.
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), "6").View().Content

	// Assert
	requireScreen(t, view, "No pull requests are waiting on your review.", "none waiting on you")
}

func TestAFailedReviewQueueShowsTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.reviewsErr = forge.ErrUnreachable

	// Act
	view := typing(t, repo.live(t, 120, 40), "6").View().Content

	// Assert
	requireScreen(t, view, "could not reach the forge")
}

func TestTheReviewsPaneWithoutAForgeOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	// An interface with no forge seam at all.
	model := sized(t, tui.New(completeConfig(), nil, tui.Deps{}), 120, 40)

	// Act
	view := press(t, model, "6").View().Content

	// Assert
	requireScreen(t, view, "does not list the pull requests waiting on your review")
	// With no forge to ask, the pane offers neither opening nor refreshing.
	refuseScreen(t, footerLine(view), "open", "refresh")
}
