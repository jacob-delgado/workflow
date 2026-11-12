// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// An edit of a pull request is saved for the pull request it was opened on.
// The Review pane can move on to another while the save is out.

import (
	"testing"
)

// otherPullTitle is the title of the pull request the branch has since.
const otherPullTitle = "Rotate the signing keys"

func TestAnEditSavedAfterTheReviewMovedOnLeavesTheOtherPullRequestAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh looks for the branch's pull request, slowly, and finds that
	// #42 was replaced by #43 meanwhile; #42's title is edited and saved, and
	// the save answers once the pane shows #43.
	repo := newWorld()
	pane := typing(t, repo.live(t, 120, 40), "4")
	repo.pull.Number, repo.pull.Title = 43, otherPullTitle
	finding, found := withheldKeys(t, pane, repo.counting("find "), "r")

	editKeys := append([]string{"e"}, append(letters(" refined"), keyEnter)...)
	saving, saved := withheldKeys(t, finding, repo.counting("edit "), editKeys...)
	moved := handedOver(t, saving, found)

	// Act
	view := handedOver(t, moved, saved).View().Content

	// Assert
	requireScreen(t, view, "#43 "+otherPullTitle)
	refuseScreen(t, view, pullTitle+" refined")
}
