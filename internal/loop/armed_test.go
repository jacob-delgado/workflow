// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// armedAt is when the announcements these tests watch were armed.
func armedAt() time.Time {
	return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
}

func TestCIWatchReadsEachState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		state forge.CIState
		after time.Duration
		want  loop.Armed
	}{
		"running":        {state: forge.CIRunning, want: loop.ArmedWaiting},
		"a check failed": {state: forge.CIFailed, want: loop.ArmedAwaitingFix},
		"passed":         {state: forge.CIPassed, want: loop.ArmedDue},
		"no check yet": {
			state: forge.CINone, after: loop.NoCIPatience - time.Second, want: loop.ArmedAwaitingChecks,
		},
		"no check, out of patience": {state: forge.CINone, after: loop.NoCIPatience, want: loop.ArmedNoCI},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, got := loop.WatchFrom(armedAt()).Read(forge.CI{State: tt.state}, armedAt().Add(tt.after))

			// Assert
			if got != tt.want {
				t.Errorf("Read(%s) after %v = %v, want %v", tt.state.Word(), tt.after, got, tt.want)
			}
		})
	}
}

func TestCIWatchCountsPatienceFromTheLastReportedCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	// A run fails late, and the fix's run has not reported a check yet.
	failedAt := armedAt().Add(loop.NoCIPatience)
	watch, _ := loop.WatchFrom(armedAt()).Read(forge.CI{State: forge.CIFailed}, failedAt)

	// Act
	_, got := watch.Read(forge.CI{State: forge.CINone}, failedAt.Add(time.Minute))

	// Assert
	if got != loop.ArmedAwaitingChecks {
		t.Errorf("Read(none) a minute after a failed run = %v, want it still awaiting checks", got)
	}
}

func TestStillArmed(t *testing.T) {
	t.Parallel()

	merged := openPull()
	merged.State = forge.StateMerged
	closed := openPull()
	closed.State = forge.StateClosed
	other := openPull()
	other.Number = 10

	cases := map[string]struct {
		pull  forge.PullRequest
		found bool
		want  error
	}{
		"the same open pull request": {pull: openPull(), found: true, want: nil},
		"another pull request":       {pull: other, found: true, want: loop.ErrPullChanged},
		"none found":                 {pull: openPull(), found: false, want: loop.ErrPullChanged},
		"closed without merging":     {pull: closed, found: true, want: loop.ErrPullChanged},
		"merged first":               {pull: merged, found: true, want: loop.ErrMergedFirst},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := loop.StillArmed(9, tt.pull, tt.found)

			// Assert
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("StillArmed = %v, want %v", err, tt.want)
			}
		})
	}
}
