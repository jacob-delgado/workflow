// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// The interface ends once: a ctrl+c queued behind a switch that has already
// ended it changes neither where it goes nor what it says it stopped. How it
// ended says whether the command line opens it again.

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestACtrlCAfterASwitchLeavesItASwitch(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post waits for CI as the switch to web is confirmed.
	working := reposWorld()
	working.ci = []forge.CI{{State: forge.CIRunning}}
	waiting := typing(t, working.live(t, 140, 40), "5", "p", "w", reposKey, "j", switchDirKey)
	left, _ := pressed(t, waiting, keyEnter)

	// Act
	after, _ := pressed(t, left, "ctrl+c")

	// Assert
	next := after.Destination()
	if line, stopped := next.StoppedWaiting(); stopped || next.Dir() != webRoot {
		t.Errorf("after the switch, ctrl+c left for %q saying %q (%v); want the switch to %s alone",
			next.Dir(), line, stopped, webRoot)
	}
}

func TestOnlyASwitchOrAReopenAsksToOpenAgain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		leaving tui.Leaving
		want    bool
	}{
		{name: "not ended", leaving: tui.LeavingNone, want: false},
		{name: "quit", leaving: tui.LeavingQuit, want: false},
		{name: "switch", leaving: tui.LeavingSwitch, want: true},
		{name: "reopen", leaving: tui.LeavingReopen, want: true},
		{name: "first run", leaving: tui.LeavingFirstRun, want: true},
	}
	for _, ending := range cases {
		t.Run(ending.name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := ending.leaving.Reopens(); got != ending.want {
				t.Errorf("an interface that ended by its %s asks to open again: %v; want %v", ending.name, got, ending.want)
			}
		})
	}
}
