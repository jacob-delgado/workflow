// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestAClickAwayAndBackLeavesTheNoticeUncovered(t *testing.T) {
	t.Parallel()

	// Arrange
	// The refresh is sent and its answer kept back, and an issue search
	// typed; a click on the Commits pane ends the search, the refresh's loss
	// then lands, and a click goes back to the Issues.
	repo := newWorld()
	repo.storeErr = errStoreFull
	refreshing, refreshed := repo.live(t, 120, 40).Update(keyMsg("r"))
	searched := typing(t, concrete(t, refreshing), "/", "a")
	away := drain(t, click(t, searched, 5, screenRow(t, searched.View().Content, "3 Commits")+1), refreshed)

	// Act
	back := click(t, away, 5, screenRow(t, away.View().Content, "1 Issues")+1)

	// Assert
	requireNotice(t, back.View().Content, issueListNotKept)
}
