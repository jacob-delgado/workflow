// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import "testing"

func TestEveryPaneShowsTheDemoWorld(t *testing.T) {
	t.Parallel()

	// Each want is drawn by the opened pane alone, never by the rail, and
	// Issues starts from another pane, since the demo opens on it: a press
	// that did nothing fails every case.
	cases := map[string]struct {
		from string
		pane string
		want []string
	}{
		"issues":       {from: "2", pane: "1", want: []string{"PROJ-412", "Redact tokens before they reach the request log"}},
		"branch":       {pane: "2", want: []string{"main", "origin/main"}},
		"commits":      {pane: "3", want: []string{"internal/wiring/reqlog.go", "internal/config/redact.go"}},
		"review":       {pane: "4", want: []string{"┌─ Review ─"}},
		"slack":        {pane: "5", want: []string{"Open a pull request first"}},
		"reviews":      {pane: "6", want: []string{"Retry the forge search once on a gateway timeout"}},
		"tasks":        {pane: "7", want: []string{"Renew the staging TLS certificate"}},
		"summary":      {pane: "8", want: []string{"route every write through Redact"}},
		"repositories": {pane: "9", want: []string{"~/src/web", "github.com/acme/api"}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			demo := opened(t)
			if tt.from != "" {
				demo.press(tt.from)
			}

			// Act
			demo.press(tt.pane)

			// Assert
			requireScreen(t, demo.screen(), tt.want...)
		})
	}
}
