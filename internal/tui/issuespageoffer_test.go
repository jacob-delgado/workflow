// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// A further page of the Issues list is offered, and asked for, only of a list
// its latest search found, with no search and no page out: a page asks from
// where the list stands, which a search or a page still out may change, and a
// list whose refresh failed is one the tracker no longer says.

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// loadMoreOffer is the footer's offer of a further page.
const loadMoreOffer = "ctrl+n load more"

// pagedIssuesWorld is a view of five issues the tracker answers three at a
// time.
func pagedIssuesWorld() *world {
	repo := newWorld()
	repo.pageSize = 3
	repo.issues = manyIssues(5)

	return repo
}

// requireNoFurtherPage fails the test when the footer offers a further page,
// or the tracker was searched more than searches times.
func requireNoFurtherPage(t *testing.T, view string, searches, want int) {
	t.Helper()

	if footer := footerLine(view); strings.Contains(footer, loadMoreOffer) {
		t.Errorf("the footer offers %q, want it withheld:\n%s", loadMoreOffer, footer)
	}

	if searches != want {
		t.Errorf("searched %d time(s), want %d: no further page asked", searches, want)
	}
}

func TestAFurtherPageIsNeitherOfferedNorAskedWhileOneIsOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := pagedIssuesWorld()
	paging, _ := withheldKeys(t, repo.live(t, 200, 40), repo.counting("search "), "ctrl+n")
	before := len(repo.asked("search "))

	// Act
	view := typing(t, paging, "ctrl+n").View().Content

	// Assert
	requireNoFurtherPage(t, view, len(repo.asked("search ")), before)
}

func TestAFurtherPageIsNeitherOfferedNorAskedWhileARefreshIsOut(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := pagedIssuesWorld()
	refreshing, _ := withheldKeys(t, repo.live(t, 200, 40), repo.counting("search "), "r")
	before := len(repo.asked("search "))

	// Act
	view := typing(t, refreshing, "ctrl+n").View().Content

	// Assert
	requireNoFurtherPage(t, view, len(repo.asked("search ")), before)
}

// truncatedThenFailing is a tracker that finds the first three of five
// issues, then fails every search after, as one that goes down between the
// load and a refresh does; it counts each search it is asked.
type truncatedThenFailing struct {
	searches atomic.Int32
}

func (tracker *truncatedThenFailing) search(int) (jira.SearchResult, error) {
	if tracker.searches.Add(1) > 1 {
		return jira.SearchResult{Issues: nil, Total: 0}, jira.ErrUnreachable
	}

	issues := manyIssues(5)

	return jira.SearchResult{Issues: issues[:3], Total: len(issues)}, nil
}

func TestAFurtherPageIsNeitherOfferedNorAskedOfAListWhoseRefreshFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	tracker := &truncatedThenFailing{searches: atomic.Int32{}}
	listed := started(t, sized(t, tui.New(completeConfig(), nil, searching(tracker.search)), 200, 40))
	failed := typing(t, listed, "r")

	// Act
	view := typing(t, failed, "ctrl+n").View().Content

	// Assert
	requireScreen(t, view, "OPS-3", failedMark)
	requireNoFurtherPage(t, view, int(tracker.searches.Load()), 2)
}
