// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// CI polling backs off once a check fails: a failed check would only fail again
// at the same rate, so the interface stops asking until the next refresh rather
// than hammering the forge for as long as it runs.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// errForgeDown is a CI check that failed, for the back-off path.
var errForgeDown = errors.New("the forge is down")

func TestPollingStopsAndShowsTheErrorAfterAFailedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI is running, but every check of it fails. The interval is short enough that
	// an unbounded poll would fire many times within the test's patience.
	failing := newWorld()
	failing.ci = []forge.CI{{State: forge.CIRunning, Total: 1, Done: 0}}
	failing.ciErr = errForgeDown
	failing.ciInterval = pollTick

	// Act
	settled := failing.live(t, 120, 40)

	// Assert
	// Polling stops after the first failed check, and the pane says why.
	if checks := failing.asked("ci"); len(checks) != 1 {
		t.Errorf("CI was checked %d times, want once before backing off: %v", len(checks), checks)
	}

	requireScreen(t, settled.View().Content, errForgeDown.Error())
}
