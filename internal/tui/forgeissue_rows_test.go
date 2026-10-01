// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// forgeIssue is the forge issue listed ahead of Jira's, numbered as a forge
// numbers its issues.
const forgeIssue jira.Key = "57"

// withForgeIssue lists a forge issue ahead of the world's Jira issues; the
// branch's issue, below it, starts selected.
func withForgeIssue() *world {
	both := newWorld()
	both.issues = append([]jira.Issue{
		{Key: forgeIssue, Summary: "Typo in the README", Status: "Open", StatusCategory: categoryNew},
	}, both.issues...)

	return both
}

func TestAForgeIssueIsListedByItsNumber(t *testing.T) {
	t.Parallel()

	// Act
	view := withForgeIssue().live(t, placesViewWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, view, "#57 ")
}

func TestAForgeIssueOffersNoCommentOrLogWork(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, wideFooterWidth, placesViewHeight), upAction).View().Content

	// Assert
	footer := footerLine(view)
	refuseScreen(t, footer, "c comment")
	refuseScreen(t, footer, "w log work")
	requireScreen(t, footer, "a assign")
}

func TestAssigningAForgeIssueStartsWithYourForgeName(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, placesViewWidth, placesViewHeight), upAction, "a").View().Content

	// Assert
	requireScreen(t, view, "Assign", "jacob")
}

func TestTheIssuesListSaysWhenTheForgeCouldNotBeRead(t *testing.T) {
	t.Parallel()

	// Arrange
	missing := newWorld()
	missing.unavailable = []string{"the forge's issues"}

	// Act
	view := missing.live(t, placesViewWidth, placesViewHeight).View().Content

	// Assert
	requireScreen(t, view, "not read: the forge's issues")
}

func TestTheWherePickerOffersTheForgesIssues(t *testing.T) {
	t.Parallel()

	// Act
	view := typing(t, withForgeIssue().live(t, placesViewWidth, placesViewHeight), placeKey).View().Content

	// Assert
	requireScreen(t, view, "forge issue  1")
}
