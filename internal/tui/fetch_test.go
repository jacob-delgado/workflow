// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errFetchFailed is how git reports a fetch it could not complete.
var errFetchFailed = errors.New("fatal: could not read from remote repository")

func TestBranchingFetchesTheBaseFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	model := branching.live(t, 120, 40)

	// Act
	created := typing(t, model, "b", keyEnter)

	// Assert
	requireScreen(t, created.View().Content, "created and switched")

	if fetches := branching.asked("fetch"); len(fetches) != 1 {
		t.Errorf("fetched %d time(s), want once before branching", len(fetches))
	}
}

func TestAFailedFetchOffersToBranchFromWhatIsThere(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	branching.branch = gitrepo.Branch{Name: baseName, Base: baseRef}
	branching.fetchErr = errFetchFailed
	model := branching.live(t, 120, 40)

	// Act: create the branch, and have the fetch fail
	failed := typing(t, model, "b", keyEnter)

	// Assert: nothing is created, and the offer to branch anyway is shown
	requireScreen(t, failed.View().Content, "could not fetch; enter branches from what you already have")

	if calls := branching.asked("create"); len(calls) != 0 {
		t.Errorf("created a branch though the fetch failed: %q", calls)
	}

	// Act: branch from what is already there
	created := typing(t, failed, keyEnter)

	// Assert: the branch is created, without fetching again
	requireScreen(t, created.View().Content, "created and switched")

	if fetches := branching.asked("fetch"); len(fetches) != 1 {
		t.Errorf("fetched %d time(s), want just the one that failed", len(fetches))
	}

	if calls := branching.asked("create"); len(calls) != 1 {
		t.Errorf("create calls = %q, want one after choosing to branch anyway", calls)
	}
}
