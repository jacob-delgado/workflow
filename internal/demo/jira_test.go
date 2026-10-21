// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// worked is the issue the demo works on.
const worked jira.Key = "PROJ-412"

// The statuses the worked issue moves through.
const (
	toDo       = "To Do"
	inProgress = "In Progress"
)

func TestMovingAnIssueChangesItsStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	moves, err := deps.Jira.Transitions(t.Context(), worked)

	if err != nil || len(moves) == 0 {
		t.Fatalf("PROJ-412's moves = %+v, %v; want at least one", moves, err)
	}

	// Act
	err = deps.Jira.Transition(t.Context(), worked, moves[0], nil)
	// Assert
	if err != nil {
		t.Fatal(err)
	}

	detail, _ := deps.Jira.Issue(t.Context(), worked)
	if detail.Issue.Status != moves[0].ToStatus || detail.Issue.StatusCategory != moves[0].ToStatusCategory {
		t.Errorf("PROJ-412 after the move is %s (%s), want %s", detail.Issue.Status, detail.Issue.StatusCategory,
			moves[0].ToStatus)
	}

	listed, _ := deps.Jira.Search(t.Context(), "", 0)

	at := slices.IndexFunc(listed.Issues, func(issue jira.Issue) bool { return issue.Key == worked })
	if listed.Issues[at].Status != moves[0].ToStatus {
		t.Errorf("the list shows PROJ-412 as %s after the move", listed.Issues[at].Status)
	}
}

func TestAMoveTheIssueDoesNotOfferIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	before, _ := deps.Jira.Issue(t.Context(), worked)

	// Act
	err := deps.Jira.Transition(t.Context(), worked, jira.Transition{ID: "99", ToStatus: "Shipped"}, nil)

	// Assert
	if err == nil {
		t.Error("a move PROJ-412 does not offer was made")
	}

	if after, _ := deps.Jira.Issue(t.Context(), worked); after.Issue.Status != before.Issue.Status {
		t.Errorf("the refused move left PROJ-412 %s, want %s", after.Issue.Status, before.Issue.Status)
	}
}

func TestAnIssueTheDemoDoesNotListHasNoMoves(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	_, err := deps.Jira.Transitions(t.Context(), "PROJ-1")

	// Assert
	if err == nil {
		t.Error("an issue the demo does not list offered moves")
	}
}

func TestAnIssueTheDemoDoesNotListCannotBeMoved(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	moves, _ := deps.Jira.Transitions(t.Context(), worked)

	// Act
	err := deps.Jira.Transition(t.Context(), "PROJ-1", moves[0], nil)

	// Assert
	if !errors.Is(err, jira.ErrNotFound) {
		t.Errorf("moving an issue the demo does not list answered %v, want jira.ErrNotFound", err)
	}

	if after, _ := deps.Jira.Issue(t.Context(), worked); after.Issue.Status != toDo {
		t.Errorf("the refused move changed the first issue listed, PROJ-412, to %s", after.Issue.Status)
	}
}

func TestTodaysSummaryHoldsTheMovesTheDemoMade(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	moves, _ := deps.Jira.Transitions(t.Context(), worked)
	_ = deps.Jira.Transition(t.Context(), worked, moves[0], nil)
	start := deps.Clock().Truncate(24 * time.Hour)

	// Act
	today, err := deps.Jira.Activity(t.Context(), start, start.Add(24*time.Hour))

	// Assert
	if err != nil || len(today.Events) != 1 || today.Events[0].Kind != jira.EventMoved ||
		today.Events[0].Detail != moves[0].ToStatus {
		t.Errorf("today's Jira activity = %+v, %v; want PROJ-412 moved to %s", today, err, moves[0].ToStatus)
	}
}

func TestTheWorkedIssueOpensToDo(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()

	// Act
	detail, err := deps.Jira.Issue(t.Context(), worked)

	// Assert
	if err != nil || detail.Issue.Status != toDo || detail.Issue.StatusCategory != jira.CategoryNew {
		t.Errorf("PROJ-412 opens %s (%s), %v; want To Do, not started", detail.Issue.Status,
			detail.Issue.StatusCategory, err)
	}
}

func TestYesterdaysSummaryDoesNotMoveTheWorkedIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := standingStill().Deps()
	today := deps.Clock().Truncate(24 * time.Hour)

	// Act
	yesterday, err := deps.Jira.Activity(t.Context(), today.AddDate(0, 0, -1), today)

	// Assert
	var moved []jira.Key

	for _, event := range yesterday.Events {
		if event.Kind == jira.EventMoved {
			moved = append(moved, event.Key)
		}
	}

	if err != nil || slices.Contains(moved, worked) || len(moved) == 0 {
		t.Errorf("yesterday's Jira activity moved %v, %v; want an issue moved, and not PROJ-412", moved, err)
	}
}
