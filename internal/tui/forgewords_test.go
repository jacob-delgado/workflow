// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestTheReviewsPaneMarksEachMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	// Arrange
	gitlab := withReviews()
	gitlab.forgeKind = forge.KindGitLab

	// Act
	view := plain(typing(t, gitlab.live(t, 120, 40), "6").View().Content)

	// Assert
	requireScreen(t, view, "!12 add request retries", "!7 fix flaky redaction test")
	refuseScreen(t, view, "#12", "#7")
}

func TestAnEmptyReviewQueueOnGitLabSaysMergeRequests(t *testing.T) {
	t.Parallel()

	// Arrange
	gitlab := newWorld()
	gitlab.forgeKind = forge.KindGitLab

	// Act
	view := typing(t, gitlab.live(t, 120, 40), "6").View().Content

	// Assert
	requireScreen(t, view, "No merge requests are waiting on your review.")
	refuseScreen(t, view, "pull request")
}

func TestTheComposerEditorHelpSaysMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	// Arrange
	gitlab := withoutPull()
	gitlab.forgeKind = forge.KindGitLab

	// Act
	typing(t, gitlab.live(t, 120, 40), "4", "n", keyCtrlO)

	// Assert
	if !strings.Contains(gitlab.editHelp, "merge request description") ||
		strings.Contains(gitlab.editHelp, "pull request") {
		t.Errorf("composer editor help = %q, want GitLab's noun and never pull request", gitlab.editHelp)
	}
}

func TestTheDescriptionEditorHelpSaysMergeRequestOnGitLab(t *testing.T) {
	t.Parallel()

	// Arrange
	gitlab := newWorld()
	gitlab.forgeKind = forge.KindGitLab

	// Act
	typing(t, gitlab.live(t, 120, 40), "4", "e", keyCtrlO)

	// Assert
	if !strings.Contains(gitlab.editHelp, "merge request description") ||
		strings.Contains(gitlab.editHelp, "pull request") {
		t.Errorf("description editor help = %q, want GitLab's noun and never pull request", gitlab.editHelp)
	}
}
