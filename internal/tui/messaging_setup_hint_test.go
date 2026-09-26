// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestTheUnsetMessagingPaneSaysWhatDoctorChecks(t *testing.T) {
	t.Parallel()

	// Arrange
	// Doctor cannot check a webhook without posting to it, so the hint promises
	// only the file offline and a bot token online. The screen is wide enough to
	// keep each phrase on one line.
	cfg := completeConfig()
	cfg.Messaging = config.Messaging{}
	model := sized(t, tui.New(cfg, nil, newWorld().deps()), 200, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "5").View().Content

	// Assert
	requireScreen(t, view, "`workflow doctor` checks the file;", "`--online` also asks Slack about a bot token.")
	refuseScreen(t, view, "`workflow doctor --online` checks it")
}
