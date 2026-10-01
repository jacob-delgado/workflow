// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

var (
	// errTrackerDown is a search that could not be answered.
	errTrackerDown = errors.New("tracker down")
	// errNoMorePages is a search the fake tracker has no page left for.
	errNoMorePages = errors.New("the fake tracker has no page left")
)

// searchAsked is one search a fake tracker was asked: the query and where its
// page starts.
type searchAsked struct {
	jql     string
	startAt int
}

// fakeTracker answers each search with the next of its pages, recording what
// it was asked.
type fakeTracker struct {
	pages []jira.SearchResult
	err   error
	asked []searchAsked
}

func (f *fakeTracker) search(jql string, startAt int) (jira.SearchResult, error) {
	f.asked = append(f.asked, searchAsked{jql: jql, startAt: startAt})
	if f.err != nil {
		return jira.SearchResult{}, f.err
	}

	if len(f.pages) == 0 {
		return jira.SearchResult{}, errNoMorePages
	}

	page := f.pages[0]
	f.pages = f.pages[1:]

	return page, nil
}

// numbered is count keys PROJ-000, PROJ-001, … in sorted order.
func numbered(count int) []jira.Key {
	keys := make([]jira.Key, 0, count)
	for index := range count {
		keys = append(keys, jira.Key(fmt.Sprintf("PROJ-%03d", index)))
	}

	return keys
}

// issuesFor is a search page holding an issue for each key.
func issuesFor(keys []jira.Key, total int) jira.SearchResult {
	issues := make([]jira.Issue, 0, len(keys))
	for _, key := range keys {
		issues = append(issues, jira.Issue{Key: key})
	}

	return jira.SearchResult{Issues: issues, Total: total}
}

func TestAssignedKeysKeepsOnlyTheKeysTheTrackerReturns(t *testing.T) {
	t.Parallel()

	// Arrange
	// A tracker that ignores the query, as the forge's own issues do, answers
	// an issue that was not asked about.
	tracker := &fakeTracker{pages: []jira.SearchResult{issuesFor([]jira.Key{"A", "C"}, 2)}}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, []jira.Key{"B", "A", "B"})
	if err != nil {
		t.Fatalf("AssignedKeys returned %v, want nil", err)
	}

	// Assert
	if want := map[jira.Key]bool{"A": true}; !maps.Equal(mine, want) {
		t.Errorf("AssignedKeys = %v, want %v", mine, want)
	}

	if want := jira.KeysAssignedToMe([]jira.Key{"A", "B"}); len(tracker.asked) != 1 || tracker.asked[0].jql != want {
		t.Errorf("the tracker was asked %v, want one search for %q", tracker.asked, want)
	}
}

func TestAssignedKeysPagesUntilTheTotalIsReached(t *testing.T) {
	t.Parallel()

	// Arrange
	keys := numbered(60)

	tracker := &fakeTracker{pages: []jira.SearchResult{issuesFor(keys[:50], 60), issuesFor(keys[50:], 60)}}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, keys)
	if err != nil {
		t.Fatalf("AssignedKeys returned %v, want nil", err)
	}

	// Assert
	if len(mine) != 60 {
		t.Errorf("AssignedKeys kept %d keys, want 60", len(mine))
	}

	starts := make([]int, 0, len(tracker.asked))
	for _, asked := range tracker.asked {
		starts = append(starts, asked.startAt)
	}

	if want := []int{0, 50}; !slices.Equal(starts, want) {
		t.Errorf("the tracker was asked from %v, want %v", starts, want)
	}
}

func TestAssignedKeysWithNoKeysAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	tracker := &fakeTracker{}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, nil)

	// Assert
	if err != nil || len(mine) != 0 || mine == nil {
		t.Errorf("AssignedKeys(nil) = %v, %v, want an empty map and no error", mine, err)
	}

	if len(tracker.asked) != 0 {
		t.Errorf("the tracker was asked %v, want nothing", tracker.asked)
	}
}

func TestAssignedKeysPassesTheTrackersError(t *testing.T) {
	t.Parallel()

	// Arrange
	tracker := &fakeTracker{err: errTrackerDown}

	// Act
	_, err := loop.AssignedKeys(tracker.search, []jira.Key{"A"})

	// Assert
	if !errors.Is(err, errTrackerDown) || errors.Is(err, errNoMorePages) || err.Error() == errTrackerDown.Error() {
		t.Errorf("AssignedKeys returned %v, want the tracker's error wrapped", err)
	}
}

func TestAssignedKeysStopsAtAnEmptyPage(t *testing.T) {
	t.Parallel()

	// Arrange
	// A tracker whose total overstates what it holds would otherwise be asked
	// forever.
	tracker := &fakeTracker{pages: []jira.SearchResult{issuesFor(nil, 100)}}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, []jira.Key{"A"})
	if err != nil {
		t.Fatalf("AssignedKeys returned %v, want nil", err)
	}

	// Assert
	if len(mine) != 0 || len(tracker.asked) != 1 {
		t.Errorf("AssignedKeys = %v after %d searches, want nothing after one", mine, len(tracker.asked))
	}
}

func TestAssignedKeysAsksInBatchesOfAHundredKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	// The query rides in a URL, so a hundred keys at a time: the first batch
	// answers over two pages, the second in one.
	keys := numbered(150)

	tracker := &fakeTracker{pages: []jira.SearchResult{
		issuesFor(keys[:50], 100), issuesFor(keys[50:100], 100), issuesFor(keys[100:], 50),
	}}

	// Act
	// Duplicates are asked about once, so they cannot push a key into a
	// batch of its own.
	mine, err := loop.AssignedKeys(tracker.search, append(slices.Clone(keys), keys[:120]...))
	if err != nil {
		t.Fatalf("AssignedKeys returned %v, want nil", err)
	}

	// Assert
	want := []searchAsked{
		{jql: jira.KeysAssignedToMe(keys[:100]), startAt: 0},
		{jql: jira.KeysAssignedToMe(keys[:100]), startAt: 50},
		{jql: jira.KeysAssignedToMe(keys[100:]), startAt: 0},
	}
	if !slices.Equal(tracker.asked, want) || len(mine) != 150 {
		t.Errorf("AssignedKeys kept %d keys after asking %v, want 150 after %v", len(mine), tracker.asked, want)
	}
}

// jiraIssue is a Jira key asked about beside a forge issue's number.
const jiraIssue jira.Key = "OPS-1"

func TestAssignedKeysNamesNoForgeNumberInTheQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	// Jira and the forge as one tracker: the forge's assigned issues lead
	// whatever page Jira answers the query with.
	tracker := &fakeTracker{pages: []jira.SearchResult{issuesFor([]jira.Key{"42", jiraIssue}, 2)}}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, []jira.Key{jiraIssue, "42"})

	// Assert
	want := jira.KeysAssignedToMe([]jira.Key{jiraIssue})
	if err != nil || len(tracker.asked) != 1 || tracker.asked[0].jql != want {
		t.Fatalf("asked %v, %v; want one search for %q", tracker.asked, err, want)
	}

	if want := map[jira.Key]bool{"42": true, jiraIssue: true}; !maps.Equal(mine, want) {
		t.Errorf("AssignedKeys = %v, want %v", mine, want)
	}
}

func TestAssignedKeysOfForgeNumbersAloneAsksWithNoQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	tracker := &fakeTracker{pages: []jira.SearchResult{issuesFor([]jira.Key{"42"}, 1)}}

	// Act
	mine, err := loop.AssignedKeys(tracker.search, []jira.Key{"7", "42"})

	// Assert
	if err != nil || len(tracker.asked) != 1 || tracker.asked[0].jql != jira.NoKeys {
		t.Fatalf("asked %v, %v; want one search with no query", tracker.asked, err)
	}

	if want := map[jira.Key]bool{"42": true}; !maps.Equal(mine, want) {
		t.Errorf("AssignedKeys = %v, want %v", mine, want)
	}
}
