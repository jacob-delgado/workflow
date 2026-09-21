// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

// issueBrowse is the browse URL the fake Jira builds for the selected issue.
const issueBrowse = "https://jira.example.com/browse/" + issueKey

func TestOpenOnAnIssueOpensItsBrowsePage(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), "o")

	// Assert
	if got := repo.asked("browse " + issueBrowse); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the selected issue", got)
	}
}

func TestCopyOnAnIssueCopiesItsBrowseURL(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), "y").View().Content

	// Assert
	if got := repo.asked("copy " + issueBrowse); len(got) != 1 {
		t.Errorf("copy calls = %v, want one for the selected issue", got)
	}

	requireScreen(t, view, "copied "+issueBrowse)
}

func TestAFailedLinkOpenShowsTheReason(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	repo.openURLErr = errNoOpener

	// Act
	view := typing(t, repo.live(t, 120, 40), "o").View().Content

	// Assert
	requireScreen(t, view, "executable file not found")
}

func TestOpenOnTheReviewPaneOpensThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	typing(t, repo.live(t, 120, 40), "4", "o")

	// Assert
	if got := repo.asked("browse " + pullURL); len(got) != 1 {
		t.Errorf("browse calls = %v, want one for the branch's pull request", got)
	}
}

func TestCopyOnTheReviewPaneCopiesThePullRequestURL(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()

	// Act
	view := typing(t, repo.live(t, 120, 40), "4", "y").View().Content

	// Assert
	if got := repo.asked("copy " + pullURL); len(got) != 1 {
		t.Errorf("copy calls = %v, want one for the branch's pull request", got)
	}

	requireScreen(t, view, "copied "+pullURL)
}

func TestTheReviewPaneOffersNoCopyWithoutAPullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := withoutPull()

	// Act
	view := typing(t, repo.live(t, 120, 40), "4").View().Content

	// Assert
	// There is no pull request URL to act on, so the copy verb is not offered.
	refuseScreen(t, footerLine(view), "copy url")
}
