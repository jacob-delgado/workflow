// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// withAFailedJob is the world with CI failed on one job whose log the forge
// keeps, listed after a passing check.
func withAFailedJob(logged bool) *world {
	failing := newWorld()
	failing.ci = []forge.CI{{State: forge.CIFailed, Total: 2, Done: 2, Failed: 1, Checks: []forge.Check{
		{Name: "build-docs", State: forge.CIPassed, URL: "https://ci/docs"},
		{ID: "501", Name: "unit-race", State: forge.CIFailed, URL: "https://ci/501", LogAvailable: logged},
	}}}

	return failing
}

func TestLOnAFailedCheckShowsTheEndOfItsLog(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withAFailedJob(true)

	// Act
	view := typing(t, failing.live(t, 120, 40), "4", "c", downAction, "l").View().Content

	// Assert
	requireScreen(t, view, "--- FAIL: TestRetry", "retry_test.go:41: got 4", "earlier lines")

	if asked := failing.asked("job-log"); len(asked) != 1 || asked[0] != "job-log 501" {
		t.Errorf("asked %q, want the one failed job's log", asked)
	}
}

func TestLOnACheckWithNoLogAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withAFailedJob(false)

	// Act
	view := typing(t, failing.live(t, 120, 40), "4", "c", downAction, "l").View().Content

	// Assert
	requireScreen(t, view, "keeps no log")

	if asked := failing.asked("job-log"); len(asked) != 0 {
		t.Errorf("asked %q, want nothing asked of a check with no log", asked)
	}
}
