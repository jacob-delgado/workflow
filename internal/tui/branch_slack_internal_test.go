// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

func TestABranchChangeClearsAStaleSlackError(t *testing.T) {
	t.Parallel()

	// Arrange
	model := New(config.Config{}, nil, Deps{})
	model.branch.branch.Name = "feat/old"
	model.slack.err, model.slack.author, model.slack.dropped = errForgeDown, "ana", "gave up"

	// Act
	updated, _ := branchLoaded{branch: gitrepo.Branch{Name: "feat/new"}}.apply(model)

	// Assert
	if s := updated.slack; s.err != nil || s.author != "" || s.dropped != "" {
		t.Errorf("a branch change left stale Slack state: %+v", s)
	}
}
