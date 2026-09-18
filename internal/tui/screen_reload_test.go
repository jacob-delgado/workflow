// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Reload offers and notice persistence, split from screen_test.go.

import (
	"testing"
)

func TestReloadIsOfferedInEveryPaneThatReloads(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ pane string }{
		"the Issues pane":  {pane: "1"},
		"the Branch pane":  {pane: "2"},
		"the Commits pane": {pane: "3"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			model := newWorld().live(t, 160, 40)

			// Act
			view := typing(t, model, tt.pane).View()

			// Assert
			requireScreen(t, footerLine(view), "r refresh")
		})
	}
}

func TestANoticeSurvivesMovingAround(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	dry.branch.Ahead = 1
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())
	pushed := typing(t, model, "2", "P", keyEnter)

	// Act
	after := typing(t, pushed, "j", "k", keyTab)
	view := after.View()

	// Assert
	requireScreen(t, view, "dry run: would push "+featureName)
	requireScreen(t, footerLine(view), "keys")
}
