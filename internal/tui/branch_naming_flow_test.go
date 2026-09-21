// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Branching names the issue throughout, split from branch_test.go.

import (
	"testing"
)

func TestBranchingForAnIssueNamesTheIssueThroughout(t *testing.T) {
	t.Parallel()

	// Arrange
	branching := newWorld()
	onIssues := branching.live(t, 160, 40)

	// Act & Assert: the Issues pane key names the issue
	requireScreen(t, footerLine(onIssues.View().Content), "b branch for PROJ-412")

	// Act: open the creator
	creator := typing(t, onIssues, "b")

	// Assert: its title names the issue
	requireScreen(t, creator.View().Content, "┏━ New branch for PROJ-412")

	// Act: create the branch
	created := typing(t, creator, keyEnter)

	// Assert: the notice says both things that happened
	requireScreen(t, created.View().Content, "● created and switched to fix/PROJ-412-fix-token-redaction")
}
