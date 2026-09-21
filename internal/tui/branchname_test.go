// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheProposedBranchNameFollowsTheConfiguredTemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := completeConfig()
	cfg.Branch = config.Branch{Template: "{prefix}/{key}", Prefixes: map[string]string{"bug": "bugfix"}}
	model := sized(t, tui.New(cfg, nil, newWorld().deps()), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "b").View().Content

	// Assert
	requireScreen(t, view, "bugfix/"+issueKey)
}

func TestTheProposedBranchNameKeepsTheDefaultWithoutConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)

	// Act
	view := typing(t, model, "2", "b").View().Content

	// Assert
	requireScreen(t, view, "fix/"+issueKey)
}
