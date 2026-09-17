// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
)

// errLinkFailed is how Jira refuses a remote link.
var errLinkFailed = errors.New("the credential was not accepted")

func TestOpeningAPullRequestOffersToLinkItOnTheIssue(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()

	// Act: open the pull request
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Assert: the confirmation to link it on the issue is shown
	requireScreen(t, opened.View(), "Link on PROJ-412", "Add this pull request's link to PROJ-412?", "#42")

	// Act: confirm the link
	linked := typing(t, opened, keyEnter)

	// Assert: Jira was asked to link it, and the outcome is reported
	requireScreen(t, linked.View(), "linked #42 on PROJ-412")

	want := "link PROJ-412 " + pullURL + " " + pullTitle
	if calls := linking.asked("link"); len(calls) != 1 || calls[0] != want {
		t.Errorf("link calls = %q, want %q", calls, want)
	}
}

func TestSkippingTheLinkAddsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	linking := withoutPull()
	opened := typing(t, linking.live(t, 120, 40), "4", "n", keyEnter)

	// Act
	skipped := typing(t, opened, keyEsc)

	// Assert
	refuseScreen(t, skipped.View(), "Add this pull request's link")

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
	requireScreen(t, failed.View(), "Link on PROJ-412", errLinkFailed.Error())
}

func TestADryRunSaysItWouldLinkThePullRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := withoutPull()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View()

	// Assert
	requireScreen(t, view, "and link it on")

	if calls := append(dry.asked("open"), dry.asked("link")...); len(calls) != 0 {
		t.Errorf("a dry run opened or linked: %q", calls)
	}
}
