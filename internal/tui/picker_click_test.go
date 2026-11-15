// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

func TestAClickWhileAMoveIsSentChoosesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: workflowMoves()}
	sending, _ := pressed(t, openPicker(t, jiraScreen(t, fake.deps(twoIssues()))), keyEnter)
	done := screenRow(t, sending.View().Content, "Done")

	// Act
	clicked := click(t, sending, detailColumn+10, done)

	// Assert
	if clicked.View().Content != sending.View().Content {
		t.Errorf("a click while the move is sent changed the screen:\n%s", clicked.View().Content)
	}
}

func TestAClickWhileAMovesFieldsAreFilledChoosesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := &fakeJira{moves: []jira.Transition{resolveIssue(), transition("31", "Done", "Done", "done")}}
	opened := openPicker(t, jiraScreen(t, fake.deps(twoIssues())))
	done := screenRow(t, opened.View().Content, "● Done")
	filling := press(t, opened, keyEnter)

	// Act
	clicked := click(t, filling, detailColumn+10, done)

	// Assert
	requireScreen(t, press(t, clicked, keyEsc).View().Content, resolveChosen)
}
