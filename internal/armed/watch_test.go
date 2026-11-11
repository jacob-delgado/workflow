// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// branchName is the branch whose pull request these tests watch.
const branchName = "fix/PROJ-7-redact-the-token"

// errSeam is a read that failed.
var errSeam = errors.New("the seam failed")

// armedAt is when the announcements these tests watch were armed.
func armedAt() time.Time {
	return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
}

// openPull is the open pull request #9 these tests arm an announcement for.
func openPull() forge.PullRequest {
	return forge.PullRequest{Number: 9, URL: "https://github.com/acme/widgets/pull/9", Title: "Redact the token"}
}

func TestCIWatchReadsEachState(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		state forge.CIState
		after time.Duration
		want  armed.Verdict
	}{
		"running":        {state: forge.CIRunning, want: armed.VerdictWaiting},
		"a check failed": {state: forge.CIFailed, want: armed.VerdictAwaitingFix},
		"passed":         {state: forge.CIPassed, want: armed.VerdictDue},
		"no check yet": {
			state: forge.CINone, after: armed.NoCIPatience - time.Second, want: armed.VerdictAwaitingChecks,
		},
		"no check, out of patience": {state: forge.CINone, after: armed.NoCIPatience, want: armed.VerdictNoCI},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, got := armed.WatchFrom(armedAt()).Read(forge.CI{State: tt.state}, armedAt().Add(tt.after))

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
	failedAt := armedAt().Add(armed.NoCIPatience)
	watch, _ := armed.WatchFrom(armedAt()).Read(forge.CI{State: forge.CIFailed}, failedAt)

	// Act
	_, got := watch.Read(forge.CI{State: forge.CINone}, failedAt.Add(time.Minute))

	// Assert
	if got != armed.VerdictAwaitingChecks {
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
		"another pull request":       {pull: other, found: true, want: armed.ErrPullChanged},
		"none found":                 {pull: openPull(), found: false, want: armed.ErrPullChanged},
		"closed without merging":     {pull: closed, found: true, want: armed.ErrPullChanged},
		"merged first":               {pull: merged, found: true, want: armed.ErrMergedFirst},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := armed.StillArmed(9, tt.pull, tt.found)

			// Assert
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Errorf("StillArmed = %v, want %v", err, tt.want)
			}
		})
	}
}
