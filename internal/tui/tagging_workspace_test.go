// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Links are kept per Slack workspace, so a token whose workspace Slack will
// not name tags nobody: the announcement says why and posts untagged, and
// People and groups says why it shows no one.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAnUnknownWorkspaceIsNamedAndThePostGoesUntagged(t *testing.T) {
	t.Parallel()

	// Arrange
	unknown := taggingWorld()
	unknown.slack.workspaceErr = messaging.ErrNoWorkspace

	// Act: open the preview
	preview := typing(t, unknown.live(t, 140, 40), "5", "p")

	// Assert: why is said, and no link is offered
	requireScreen(t, preview.View().Content,
		"can't tell which Slack workspace this token is for", "this posts untagged")
	refuseScreen(t, footerLine(preview.View().Content), "link to Slack")

	// Act: post
	typing(t, preview, keyEnter)

	// Assert: the post goes, untagged, and no choice is remembered
	if got := postedText(t, unknown); strings.Contains(got, "cc ") {
		t.Errorf("posted %q, want it untagged", got)
	}

	if calls := unknown.asked("record-groups "); len(calls) != 0 {
		t.Errorf("recorded groups %q for an untagged post", calls)
	}
}

func TestPeopleAndGroupsSaysWhenTheWorkspaceIsUnknown(t *testing.T) {
	t.Parallel()

	// Arrange
	unknown := taggingWorld()
	unknown.slack.workspaceErr = messaging.ErrNoWorkspace

	// Act
	view := openPeople(t, unknown).View().Content

	// Assert
	requireScreen(t, view, "can't tell which Slack workspace this token is for")
	refuseScreen(t, view, "→ Carla Diaz")
}
