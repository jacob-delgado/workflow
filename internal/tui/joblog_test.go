// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"slices"
	"strings"
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

func TestScrollingUpALogThatFitsKeepsItsLastLine(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withAFailedJob(true)

	// Act
	view := typing(t, failing.live(t, 120, 40), "4", "c", downAction, "l", upAction).View().Content

	// Assert
	requireScreen(t, view, "--- FAIL: TestRetry", "retry_test.go:41: got 4")
}

// Scrolling stops with the log's first line at the top, the window still
// full, so the first step back down moves it at once.
func TestScrollingUpPastTheTopOfALogStopsThere(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withAFailedJob(true)

	lines := make([]string, 0, 100)
	for index := range 100 {
		lines = append(lines, fmt.Sprintf("line %03d", index))
	}

	failing.jobLog = forge.JobLog{Text: strings.Join(lines, "\n")}
	opened := typing(t, failing.live(t, 120, 40), "4", "c", downAction, "l")

	// Act
	view := typing(t, opened, append(slices.Repeat([]string{upAction}, 200), downAction)...).View().Content

	// Assert
	requireScreen(t, view, "line 001", "line 020")
	refuseScreen(t, view, "line 000")
}

func TestASCIIModeSeparatesAFailedChecksPartsInASCII(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"its stage and name": {keys: []string{"4"}, want: "test - unit-race"},
		"its log's title":    {keys: []string{"4", "c", downAction, "l"}, want: "unit-race - log"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			failing := withAFailedJob(true)
			failing.ci[0].Checks[1].Stage = "test"

			// Act
			view := typing(t, asciiInterface(t, failing, 120, 40), tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

// A CI read that fails says so, and lists no failed check from the read
// before it, which may since have passed.
func TestAFailedCIReadListsNoChecksFromTheReadBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := withAFailedJob(true)
	shown := typing(t, failing.live(t, 120, 40), "4")
	failing.ciErr = forge.ErrUnreachable

	// Act
	view := typing(t, shown, "r").View().Content

	// Assert
	requireScreen(t, view, "could not reach the forge")
	refuseScreen(t, view, "unit-race")
}
