// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestTheSpaceKeyTypesASpaceIntoTheIssueFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	// A terminal can report the space bar as the space key with no text, and
	// the filter must still read it as a space.
	listed := issuesScreen(t, assigned(
		issue("OPS-1", "Fix issue", "indeterminate"), issue("OPS-2", "Fix bug", "new")))

	// Act
	filtered := typing(t, listed, "/", "F", "i", "x", keySpace, "b")

	// Assert
	requireScreen(t, filtered.View().Content, "OPS-2 In Progress Fix bug", "search: Fix b")
	refuseScreen(t, filtered.View().Content, "OPS-1")
}

func TestAKeyThatTypesNothingLeavesTheIssueFilterAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	listed := issuesScreen(t, assigned(
		issue("OPS-1", "Fix issue", "indeterminate"), issue("OPS-2", "Fix bug", "new")))
	filtering := typing(t, listed, "/", "F", "i", "x")

	// Act
	after := typing(t, filtering, keyTab)

	// Assert
	// Still open with the same text: tab neither closed the filter nor typed
	// a character that would match neither issue.
	requireScreen(t, after.View().Content, "OPS-1 In Progress Fix issue", "OPS-2 In Progress Fix bug", "search: Fix")
	refuseScreen(t, after.View().Content, "no issue matches the filters")
}
