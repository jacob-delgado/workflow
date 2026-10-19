// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/activity"
	"github.com/jacob-delgado/workflow/internal/demo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// standingStill is a world whose clock never moves on from the demo's epoch.
func standingStill() *demo.World {
	return demo.New(func() time.Duration { return 0 })
}

func TestAnIssueTheDemoDoesNotListIsNotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	world := standingStill()

	// Act
	_, err := world.Deps().Jira.Issue(t.Context(), "PROJ-1")

	// Assert
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("reading an issue the demo does not list answered %v, want jira.ErrNotFound", err)
	}
}

func TestTodaysSummaryHoldsNothingBeforeTheDemoActs(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	today := activity.DateOf(deps.Clock())

	period, err := activity.NewPeriod(today, today)
	if err != nil {
		t.Fatal(err)
	}

	start, end := period.Bounds(time.UTC)

	// Act
	commits := deps.Git.CommitsBetween(t.Context(), start, end)
	moved, jiraErr := deps.Jira.Activity(t.Context(), start, end)
	opened, forgeErr := deps.Forge.Activity(t.Context(), start, end)

	// Assert
	if len(commits) != 1 || len(commits[0].Commits) != 0 {
		t.Errorf("today's commits = %+v, want none in the one repository", commits)
	}

	if len(moved.Events) != 0 || jiraErr != nil {
		t.Errorf("today's Jira activity = %+v, %v, want none", moved, jiraErr)
	}

	if len(opened.Events) != 0 || forgeErr != nil {
		t.Errorf("today's forge activity = %+v, %v, want none", opened, forgeErr)
	}
}

func TestTheDemosViewListsNoIssueDone(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	listed, err := deps.Jira.Search(t.Context(), "", 0)

	// Assert
	// The default view is the issues assigned to you and not done
	// (jira.AssignedToMe), so a done issue in it is one no real view lists.
	for _, issue := range listed.Issues {
		if issue.StatusCategory == jira.CategoryDone {
			t.Errorf("the view lists %s, which is done (%s)", issue.Key, issue.Status)
		}
	}

	if err != nil || len(listed.Issues) == 0 {
		t.Errorf("the view = %+v, %v; want its issues", listed, err)
	}
}
