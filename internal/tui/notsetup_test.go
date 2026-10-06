// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// notStartedGlyph is the not-started mark the default (unicode) glyph set
// draws, the one guidance about what is not set up opens with.
const notStartedGlyph = "○"

// refuseRedGlyph fails the test if any row draws the not-started mark in the
// failure color: guidance is never red.
func refuseRedGlyph(t *testing.T, view string) {
	t.Helper()

	if strings.Contains(view, redOpen()+notStartedGlyph) {
		t.Errorf("the not-started mark is drawn in the failure color:\n%q", view)
	}
}

func TestTheReviewPaneWithNoForgeTokenSaysHowToSetOneUp(t *testing.T) {
	t.Parallel()

	// Arrange
	noToken := withoutPull()
	noToken.pullErr = fmt.Errorf("%w: %s", forge.ErrNoToken, forge.Sources(forge.KindGitHub, "github.com"))

	// Act
	view := typing(t, noToken.live(t, 120, 40), "4").View().Content

	// Assert
	requireScreen(t, view, notStartedGlyph+" no forge token found: set $GITHUB_TOKEN",
		notStartedGlyph+" no forge token")
	refuseScreen(t, view, failGlyph)
	refuseRedGlyph(t, view)
}

func TestTheIssuesPaneWithNoJiraTokenSaysHowToSetOneUp(t *testing.T) {
	t.Parallel()

	// Arrange
	search := failing(fmt.Errorf("searching: %w", jira.ErrNoCredential))

	// Act
	view := issuesScreen(t, search).View().Content

	// Assert
	requireScreen(t, view, notStartedGlyph+" Jira has no token; set jira.token", notStartedGlyph+" not set up")
	refuseScreen(t, view, failGlyph)
	refuseRedGlyph(t, view)
}

func TestTheIssuesPaneStillSaysAJiraRefusalBroke(t *testing.T) {
	t.Parallel()

	// Arrange
	search := failing(fmt.Errorf("searching: %w", jira.ErrUnauthorized))

	// Act
	view := issuesScreen(t, search).View().Content

	// Assert
	requireFailureRow(t, view, "Jira did not accept the token.")
	refuseScreen(t, view, notStartedGlyph+" Jira")
}
