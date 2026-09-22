// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheIssueListSeedsFromTheCache(t *testing.T) {
	t.Parallel()

	// Arrange
	// The store cached an issue list; the pane shows it at once. The model is not
	// driven through Init here, so nothing but the cache has filled the list.
	starting := newWorld()
	starting.cachedIssues = []jira.Issue{
		{
			Key: "CACHE-1", Summary: "From the cache",
			Status: statusInProgress, StatusCategory: categoryIndeterminate, Type: "Bug",
		},
	}

	// Act
	view := sized(t, tui.New(starting.cfg, nil, starting.deps()), 120, 40).View().Content

	// Assert
	requireScreen(t, view, "CACHE-1", "From the cache")
}

func TestASearchCachesTheIssueList(t *testing.T) {
	t.Parallel()

	// Arrange
	// The world's tracker returns two issues; loading them caches the list.
	searching := newWorld()

	// Act
	searching.live(t, 120, 40)

	// Assert
	if calls := searching.asked("cache"); len(calls) != 1 || calls[0] != "cache 2" {
		t.Errorf("cache calls = %q, want the two loaded issues cached", calls)
	}
}
