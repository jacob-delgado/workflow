// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// errLinkFailed is how Jira refuses a remote link.
var errLinkFailed = errors.New("the credential was not accepted")

// reviewTransitions offers In Progress and In Review, both indeterminate, so a
// test can tell "the one named In Review" from "the first in-progress one".
func reviewTransitions() []jira.Transition {
	return []jira.Transition{
		transition("11", "Start Progress", statusInProgress, categoryIndeterminate),
		transition("31", "Start Review", statusInReview, categoryIndeterminate),
	}
}

func TestLinkingAPullRequestThenOffersTheReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issue moves to In Review once its pull request is open.
	linking := withoutPull()
	linking.cfg.Jira.ReviewStatus = statusInReview
	linking.moves = reviewTransitions()
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	linked := typing(t, opened, keyEnter)

	// Assert
	// In Review shares the indeterminate category with In Progress, so the offer
	// is pre-selected by name, not by category.
	requireScreen(t, linked.View().Content, "Change status", "▸ ◐ Start Review")
}

func TestSkippingTheLinkStillOffersTheReviewStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()
	linking.cfg.Jira.ReviewStatus = statusInReview
	linking.moves = reviewTransitions()
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	skipped := typing(t, opened, keyEsc)

	// Assert
	requireScreen(t, skipped.View().Content, "Change status", "▸ ◐ Start Review")
}

func TestNoReviewOfferWhenJiraDoesNotHaveTheStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	// The issue's workflow has no transition to the configured review status, so
	// there is nothing to move to — the picker must not open on an unrelated one.
	linking := withoutPull()
	linking.cfg.Jira.ReviewStatus = statusInReview
	linking.moves = []jira.Transition{transition("31", "Done", "Done", "done")}
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	skipped := typing(t, opened, keyEsc)

	// Assert
	refuseScreen(t, skipped.View().Content, "Change status")
}

func TestOpeningAPullRequestOffersToLinkItOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()

	// Act: open the pull request
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Assert: the confirmation to link it on the issue is shown
	requireScreen(t, opened.View().Content, "Link on PROJ-412", "Add this pull request's link to PROJ-412?", "#42")

	// Act: confirm the link
	linked := typing(t, opened, keyEnter)

	// Assert: Jira was asked to link it, and the outcome is reported
	requireScreen(t, linked.View().Content, "linked #42 on PROJ-412")

	want := "link PROJ-412 " + pullURL + " " + pullTitle
	if calls := linking.asked("link"); len(calls) != 1 || calls[0] != want {
		t.Errorf("link calls = %q, want %q", calls, want)
	}
}

func TestAForgeIssueNumberIsOfferedNoJiraLinkOrMove(t *testing.T) {
	t.Parallel()

	// Arrange
	// The branch names the forge's issue 42, not a Jira one: Jira would refuse
	// the link, or read 42 as the id of an unrelated issue.
	forgeNumber := withoutPull()
	forgeNumber.branch.Name = "fix/42-typo"
	forgeNumber.cfg.Jira.ReviewStatus = statusInReview
	forgeNumber.moves = reviewTransitions()

	// Act
	view := typing(t, forgeNumber.live(t, 120, 40), "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "opened #42")
	refuseScreen(t, view, "Link on", "Change status")
}

func TestSkippingTheLinkAddsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	skipped := typing(t, opened, keyEsc)

	// Assert
	refuseScreen(t, skipped.View().Content, "Add this pull request's link")

	if calls := linking.asked("link"); len(calls) != 0 {
		t.Errorf("linked though the offer was skipped: %q", calls)
	}
}

func TestAFailedLinkKeepsTheConfirmationOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()
	linking.linkErr = errLinkFailed
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	failed := typing(t, opened, keyEnter)

	// Assert
	requireScreen(t, failed.View().Content, "Link on PROJ-412", errLinkFailed.Error())
}

func TestADryRunSaysItWouldLinkThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "and link it on")

	if calls := append(dry.asked("open"), dry.asked("link")...); len(calls) != 0 {
		t.Errorf("a dry run opened or linked: %q", calls)
	}
}

func TestADryRunOffersNoJiraLinkForAForgeIssueNumber(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	dry.branch.Name = "fix/42-typo"
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would")
	refuseScreen(t, view, "and link it on")
}
