// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheIssuesPaneShowsTheCachedListBeforeJiraAnswers(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := searching(assigned(issue("NEW-9", "Fresh from Jira", "new")))
	deps.Cache.LoadIssues = func() []jira.Issue {
		return []jira.Issue{issue("OPS-1", "Fix login", "indeterminate")}
	}

	// Act
	view := sized(t, tui.New(completeConfig(), nil, deps), 120, 40).View()

	// Assert
	// Painted before Init runs: the cached row is there, not the Issues pane's
	// own loading line (marked by its heavy border).
	requireScreen(t, view, "OPS-1 Fix login")
	refuseScreen(t, view, "┃ loading…")
}

func TestAFreshLoadReplacesTheCachedList(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := searching(assigned(issue("NEW-9", "Fresh from Jira", "new")))
	deps.Cache.LoadIssues = func() []jira.Issue {
		return []jira.Issue{issue("OPS-1", "Stale", "indeterminate")}
	}

	// Act
	view := started(t, sized(t, tui.New(completeConfig(), nil, deps), 120, 40)).View()

	// Assert
	requireScreen(t, view, "NEW-9 Fresh from Jira")
	refuseScreen(t, view, "OPS-1")
}

func TestALoadedListIsCachedForNextTime(t *testing.T) {
	t.Parallel()

	// Arrange
	saving := newWorld()

	// Act
	saving.live(t, 120, 40)

	// Assert
	if calls := saving.asked("cache-save"); len(calls) != 1 {
		t.Errorf("cache-save calls = %q, want the loaded list saved once", calls)
	}

	if saved := saving.saved(); len(saved) != len(saving.issues) {
		t.Errorf("saved %d issues, want the %d that loaded", len(saved), len(saving.issues))
	}
}
