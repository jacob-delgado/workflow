// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

func TestAnEscOnAnotherBranchKeepsThePullRequestDraftOfTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// The title is typed over on the first branch and kept by esc; the
	// composer is opened and closed on another branch; the checkout comes
	// back.
	repo := withoutPull()
	kept := typing(t, repo.live(t, 120, 40), "4", "n", keyEnter, keyBackspace, "!", keyEsc)
	repo.branch.Name = otherFeatureBranch
	looked := typing(t, kept, "2", "r", "4", "n", keyEsc)
	repo.branch.Name = featureName
	back := typing(t, looked, "2", "r")

	// Act
	view := typing(t, back, "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "> "+pullTitle[:len(pullTitle)-1]+"!")
}

func TestAPullRequestOpenedOnAnotherBranchKeepsTheDraftOfTheFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	// A title is typed on the first branch and kept by esc; the checkout
	// moves to another branch, which opens its pull request, the offer to
	// link it on the issue skipped.
	repo := withDirectories(withoutPull())
	kept := typingIntoAPullRequest(pasted(" now"))(t, repo)
	repo.branch.Name = otherFeatureBranch
	opened := typing(t, kept, "2", "r", "4", "n", keyEnter, keyEnter, keyEsc)

	// Act
	view := switchingToWeb(t, opened)

	// Assert
	requireScreen(t, view, "Switch to ~/src/web", lostPullRequest)

	if opens := repo.asked("open "); len(opens) != 1 {
		t.Errorf("open calls = %q, want the other branch's pull request", opens)
	}
}

func TestAPullRequestOpenedAfterItsPushDropsItsDraft(t *testing.T) {
	t.Parallel()

	// Arrange
	// A title is typed and the branch, not yet pushed, is pushed before the
	// pull request opens, keeping the draft while the push is out; the
	// offer to link it on the issue is skipped.
	repo := withDirectories(withoutPull())
	repo.branch.Upstream = ""
	opened := typing(t, repo.live(t, 120, 40), "4", "n", keyEnter, pasted(" now"), keyEnter, keyEsc)

	// Act
	view := switchingToWeb(t, opened)

	// Assert
	requireScreen(t, view, "Switch to ~/src/web")
	refuseScreen(t, view, lostPullRequest)

	if opens := repo.asked("open "); len(opens) != 1 {
		t.Errorf("open calls = %q, want the pull request opened after the push", opens)
	}
}
