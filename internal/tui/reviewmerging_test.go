// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// mergingBranch is newWorld with the branch's merge request locked while
// GitLab merges it.
func mergingBranch() *world {
	w := newWorld()
	w.pull.State = forge.StateMerging

	return w
}

func TestTheReviewPaneSaysTheForgeIsMergingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	merging := mergingBranch()

	// Act
	view := typing(t, merging.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, "◐ merging")
	refuseScreen(t, view, "CI     ")
}

func TestAPullRequestTheForgeIsMergingOffersNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	merging := mergingBranch()

	// Act
	view := typing(t, merging.live(t, 120, 40), "4").View().Content

	// Assert
	footer := footerLine(view)
	for _, offered := range []string{"open pull request", "edit", "merge", "re-run", "finish"} {
		if strings.Contains(footer, offered) {
			t.Errorf("the footer offers %q on a pull request being merged: %s", offered, footer)
		}
	}
}
