// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
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

func TestUpMovesBackThroughTheFilter(t *testing.T) {
	t.Parallel()

	// Act
	// Down to example/other, up to no repository, and check it.
	view := filtering(t, facetsWorld(), downAction, "up", keySpace)

	// Assert
	if got := listedRequests(view); !slices.Equal(got, []string{"3"}) {
		t.Errorf("listed %v, want only #3, which names no repository:\n%s", got, plain(view))
	}
}

func TestCheckingAValueTwiceUnchecksIt(t *testing.T) {
	t.Parallel()

	// Act
	view := filtering(t, facetsWorld(), downAction, keySpace, keySpace)

	// Assert
	if got := listedRequests(view); len(got) != 4 {
		t.Errorf("listed %v, want all four requests:\n%s", got, plain(view))
	}
}

func TestFOverAnEmptyQueueOpensNoFilter(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, newWorld().live(t, 120, 40), "6", filterKey).View().Content

	// Assert
	requireScreen(t, view, "No pull requests are waiting on your review.")
	refuseScreen(t, view, filterTitleBar)
}

func TestByRepositoryHeadsARepositoryOnceForAllItsRequests(t *testing.T) {
	t.Parallel()

	// Act
	view := plain(typing(t, facetsWorld().live(t, 120, 40), "6", "s", "s").View().Content)

	// Assert
	headings := 0

	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, exampleRepo) && !strings.Contains(line, "#") {
			headings++
		}
	}

	if headings != 1 {
		t.Errorf("example/repo is headed %d times, want once above #5 and #7:\n%s", headings, view)
	}

	if got := listedRequests(view); !slices.Equal(got, []string{"3", "12", "5", "7"}) {
		t.Errorf("listed %v, want no repository, example/other, then example/repo's two:\n%s", got, view)
	}
}

func TestAKeyTheFilterDoesNotUseChangesNothing(t *testing.T) {
	t.Parallel()

	// Act
	view := filtering(t, facetsWorld(), "x", keySpace)

	// Assert
	if got := listedRequests(view); !slices.Equal(got, []string{"3"}) {
		t.Errorf("listed %v, want only #3, the first value still under the cursor:\n%s", got, plain(view))
	}
}

func TestClickingARepositoryHeadingKeepsTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// By repository, with #12, under example/other, selected: the selection
	// stays on #5, the oldest, across the sorts, and up moves it.
	repo := facetsWorld()
	grouped := typing(t, repo.live(t, 120, 40), "6", "s", "s", "up")
	row := screenRow(t, grouped.View().Content, exampleRepo)

	// Act
	typing(t, click(t, grouped, 60, row), "o")

	// Assert
	if opened := repo.asked("browse"); len(opened) != 1 || !strings.Contains(opened[0], reviewOlderURL) {
		t.Errorf("opened %q, want #12, still selected after the click on a heading", opened)
	}
}

func TestClickingTheReviewsRailKeepsTheSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	// #12, the second oldest, selected.
	repo := facetsWorld()
	moved := typing(t, repo.live(t, 120, 40), "6", downAction)
	row := screenRow(t, moved.View().Content, "4 review requests waiting")

	// Act
	typing(t, click(t, moved, 5, row), "o")

	// Assert
	if opened := repo.asked("browse"); len(opened) != 1 || !strings.Contains(opened[0], reviewOlderURL) {
		t.Errorf("opened %q, want #12, still selected after the click on the rail", opened)
	}
}

func TestSortAndFilterWaitForTheQueue(t *testing.T) {
	t.Parallel()

	// Arrange
	// The Reviews pane before the forge's answer is read back.
	repo := facetsWorld()
	model := pressing(t, sized(t, tui.New(repo.cfg, nil, repo.deps()), 120, 40), "6")

	// Act
	view := plain(pressing(t, model, "s", filterKey).View().Content)

	// Assert
	requireScreen(t, view, "looking")

	if strings.Contains(view, "newest first") || strings.Contains(view, filterTitleBar) {
		t.Errorf("sorted or filtered a queue not yet read:\n%s", view)
	}
}

func TestAKeyTheReviewsPaneDoesNotUseChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := facetsWorld()

	// Act
	typing(t, repo.live(t, 120, 40), "6", "x", "o")

	// Assert
	if opened := repo.asked("browse"); len(opened) != 1 || !strings.Contains(opened[0], "/pull/5") {
		t.Errorf("opened %q, want #5, the oldest, still selected", opened)
	}
}

// filterTitleBar is the filter's title in the detail pane's top border.
const filterTitleBar = "━ Filter ━"

// pressing presses keys in order without running what each starts, so a test
// can look at the interface before an answer it asked for comes back.
func pressing(t *testing.T, model tui.Model, keys ...string) tui.Model {
	t.Helper()

	for _, key := range keys {
		updated, _ := model.Update(keyMsg(key))
		model = concrete(t, updated)
	}

	return model
}

// waitingReviewsFooter is the Reviews pane's footer before the forge's answer
// about the queue is read back.
func waitingReviewsFooter(t *testing.T, world *world) string {
	t.Helper()

	return footerLine(pressing(t, sized(t, tui.New(world.cfg, nil, world.deps()), 120, 40), "6").View().Content)
}

// readReviewsFooter is the Reviews pane's footer once the queue is read back.
func readReviewsFooter(t *testing.T, world *world) string {
	t.Helper()

	return footerLine(typing(t, world.live(t, 120, 40), "6").View().Content)
}

// sortOffer and filterOffer are the Reviews footer's sort and filter keys.
const (
	sortOffer   = "s sort"
	filterOffer = "f filter"
)

func TestTheReviewsFooterOffersSortAndFilterOnlyWhereTheyAct(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.reviewsErr = forge.ErrUnreachable

	cases := map[string]struct {
		world    *world
		footer   func(*testing.T, *world) string
		offered  []string
		withheld []string
	}{
		"while loading": {
			world: facetsWorld(), footer: waitingReviewsFooter, withheld: []string{sortOffer, filterOffer},
		},
		"once it failed": {world: failing, footer: readReviewsFooter, withheld: []string{sortOffer, filterOffer}},
		"with none waiting": {
			world: newWorld(), footer: readReviewsFooter, offered: []string{sortOffer}, withheld: []string{filterOffer},
		},
		"with some waiting": {world: facetsWorld(), footer: readReviewsFooter, offered: []string{sortOffer, filterOffer}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			footer := tt.footer(t, tt.world)

			// Assert
			requireScreen(t, footer, tt.offered...)
			refuseScreen(t, footer, tt.withheld...)
		})
	}
}
