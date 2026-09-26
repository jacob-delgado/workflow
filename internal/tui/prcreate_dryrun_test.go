// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestADryRunOpenWhereJiraTakesNoLinkOffersOnlyTheMove(t *testing.T) {
	t.Parallel()

	// Arrange
	moveOnly := withoutPull()
	moveOnly.cfg.Jira.ReviewStatus = statusInReview
	deps := moveOnly.deps()
	deps.Jira.LinkPullRequest = nil
	model := sized(t, tui.New(moveOnly.cfg, nil, deps).WithDryRun(), 200, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "4", "n", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "into main, then offer to move "+issueKey+" to "+statusInReview)
	refuseScreen(t, view, "to link")
}
