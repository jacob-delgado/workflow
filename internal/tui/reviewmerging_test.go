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

func TestAPullRequestTheForgeIsMergingIsNotOfferedToAnnounce(t *testing.T) {
	t.Parallel()

	// Arrange
	merging := mergingBranch()

	// Act
	view := typing(t, merging.live(t, 140, 40), "5", "p").View().Content

	// Assert
	refuseScreen(t, footerLine(view), "announce")
	requireScreen(t, view, "is merging; announce it once it has merged")

	if calls := merging.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request the forge is merging", calls)
	}
}

func TestAPreviewPostsNothingOnceThePullRequestItAnnouncesStartsMerging(t *testing.T) {
	t.Parallel()

	// Arrange
	// The preview opens while the pane's read is out, and that read answers
	// that the forge began merging the pull request.
	repo := newWorld()
	pane := typing(t, repo.live(t, 140, 40), "5")
	repo.pull.State = forge.StateMerging
	reading, reread := pressed(t, pane, "r")
	merging := drain(t, typing(t, reading, "p"), reread)

	// Act
	view := typing(t, merging, keyEnter).View().Content

	// Assert
	requireScreen(t, view, "began merging while the preview was open")

	if calls := repo.asked("post "); len(calls) != 0 {
		t.Errorf("posted %q for a pull request the forge began merging", calls)
	}
}

func TestAPreviewWaitsForNothingOnceThePullRequestItAnnouncesStartsMerging(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI is running. The preview opens while the pane's read is out, and that
	// read answers that the forge began merging the pull request.
	repo := newWorld()
	repo.ci = []forge.CI{{State: forge.CIRunning}}
	pane := typing(t, repo.live(t, 140, 40), "5")
	repo.pull.State = forge.StateMerging
	reading, reread := pressed(t, pane, "r")
	merging := drain(t, typing(t, reading, "p"), reread)

	// Act
	view := typing(t, merging, "w").View().Content

	// Assert
	// The preview stays open on why, rather than closing on a wait the
	// hold's first read would drop.
	requireScreen(t, view, "┏━ Announce to Slack", "began merging while the preview was open")
}

func TestAPostRefusedBeforeTheForgeBeganMergingStaysShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refused post leaves its error in the Slack pane, and then the forge
	// begins merging the pull request.
	refusing := newWorld()
	refusing.postErr = errNotInChannel
	refused := typing(t, refusing.live(t, 140, 40), "5", "p", keyEnter, keyEsc)
	refusing.pull.State = forge.StateMerging

	// Act
	view := typing(t, refused, "r").View().Content

	// Assert
	requireScreen(t, view, "is merging; announce it once it has merged", "The messaging service refused the post")
}
