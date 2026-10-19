// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
		"issues":  {from: "2", pane: "1", want: []string{"PROJ-412", "Redact tokens in the request log"}},
		"branch":  {pane: "2", want: []string{"main", "origin/main"}},
		"commits": {pane: "3", want: []string{"internal/wiring/reqlog.go", "internal/config/redact.go"}},
		"review":  {pane: "4", want: []string{"┌─ Review ─"}},
		"slack":   {pane: "5", want: []string{"Open a pull request first"}},
		"reviews": {pane: "6", want: []string{"Retry the forge search once on a gateway timeout"}},
		"tasks": {pane: "7", want: []string{
			"Renew the staging TLS certificate", "PROJ-412: Redact tokens in the request log",
			"Sam can review it once CI is green",
		}},
		"summary": {pane: "8", want: []string{
			"route every write through Redact", "Retry the forge search once on a gateway timeout",
		}},
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

// The terminal docs/media/terminal.tape films: its 900 by 520 pixels at its
// font's size.
const (
	recordingWidth  = 94
	recordingHeight = 30
)

// recorded is the demo's interface at the size the recording films.
func recorded(t *testing.T) *session {
	t.Helper()

	demo := opened(t)
	demo.drain(demo.deliver(tea.WindowSizeMsg{Width: recordingWidth, Height: recordingHeight}), settleWithin)

	return demo
}

func TestTheRecordedDiffReadsWholeAtTheRecordingsSize(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := recorded(t)

	changes, err := demo.world.Deps().Git.Changes(t.Context())
	if err != nil || len(changes) == 0 {
		t.Fatalf("the demo's changes = %v, %v; want some", changes, err)
	}

	diff, err := demo.world.Deps().Git.Diff(t.Context(), changes[0])
	if err != nil {
		t.Fatal(err)
	}

	// Act
	demo.press("3")

	// Assert
	// The pane draws a tab as four spaces.
	for _, line := range diff {
		requireScreen(t, demo.screen(), strings.ReplaceAll(line, "\t", "    "))
	}
}

func TestTheProposedBranchReadsWholeAtTheRecordingsSize(t *testing.T) {
	t.Parallel()

	// Arrange
	demo := recorded(t)

	// Act
	demo.press("1", "b")

	// Assert
	requireScreen(t, demo.screen(), "> fix/PROJ-412-redact-tokens-in-the-request-log")
}
