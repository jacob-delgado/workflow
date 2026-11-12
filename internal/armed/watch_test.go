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

func TestCIWatchCountsNoTimeAcrossAnOutage(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every read for longer than the patience fails, then one is heard.
	outage := armedAt().Add(armed.NoCIPatience + time.Minute)
	watch := armed.WatchFrom(armedAt()).Unheard()

	// Act
	_, got := watch.Read(forge.CI{State: forge.CINone}, outage)

	// Assert
	if got != armed.VerdictAwaitingChecks {
		t.Errorf("Read(none) after an outage past the patience = %v, want it still awaiting checks", got)
	}
}

func TestCIWatchCountsTheQuietReadsEitherSideOfAnOutage(t *testing.T) {
	t.Parallel()

	// Arrange
	// Six quiet minutes are heard, an hour goes unheard, then four more.
	quietAt := armedAt().Add(6 * time.Minute)
	watch, _ := armed.WatchFrom(armedAt()).Read(forge.CI{State: forge.CINone}, quietAt)
	heardAgainAt := quietAt.Add(time.Hour)
	watch, _ = watch.Unheard().Read(forge.CI{State: forge.CINone}, heardAgainAt)

	// Act
	_, got := watch.Read(forge.CI{State: forge.CINone}, heardAgainAt.Add(4*time.Minute))

	// Assert
	if got != armed.VerdictNoCI {
		t.Errorf("Read(none) after ten heard quiet minutes around an outage = %v, want it out of patience", got)
	}
}

// runningCI is CI still running, with done of its three checks finished.
func runningCI(done int) forge.CI {
	return forge.CI{State: forge.CIRunning, Total: 3, Done: done, Failed: 0, Checks: nil}
}

func TestCIWatchGivesUpOnCIThatRunsUnchangedForItsPatience(t *testing.T) {
	t.Parallel()

	// Arrange
	watch, _ := armed.WatchFrom(armedAt()).Read(runningCI(1), armedAt())

	// Act
	_, got := watch.Read(runningCI(1), armedAt().Add(armed.RunningPatience))

	// Assert
	if got != armed.VerdictRunningTooLong {
		t.Errorf("Read(running) unchanged for %v = %v, want it running too long", armed.RunningPatience, got)
	}
}

func TestCIWatchWaitsOnCIThatRunsUnchangedWithinItsPatience(t *testing.T) {
	t.Parallel()

	// Arrange
	watch, _ := armed.WatchFrom(armedAt()).Read(runningCI(1), armedAt())

	// Act
	_, got := watch.Read(runningCI(1), armedAt().Add(armed.RunningPatience-time.Second))

	// Assert
	if got != armed.VerdictWaiting {
		t.Errorf("Read(running) unchanged for just under %v = %v, want it waiting", armed.RunningPatience, got)
	}
}

func TestCIWatchCountsRunningPatienceFromTheLastChange(t *testing.T) {
	t.Parallel()

	job := forge.Check{ID: "1", Name: "test", State: forge.CIRunning}
	cases := map[string]forge.CI{
		"a check finished":      runningCI(2),
		"a check rerun, failed": {State: forge.CIRunning, Total: 3, Done: 1, Failed: 1, Checks: nil},
		"a check started":       {State: forge.CIRunning, Total: 4, Done: 1, Failed: 0, Checks: nil},
		"a job listed":          {State: forge.CIRunning, Total: 3, Done: 1, Failed: 0, Checks: []forge.Check{job}},
	}

	for name, changed := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The run changes an hour in, so its patience starts again there.
			watch, _ := armed.WatchFrom(armedAt()).Read(runningCI(1), armedAt())
			watch, _ = watch.Read(changed, armedAt().Add(time.Hour))

			// Act
			_, got := watch.Read(changed, armedAt().Add(armed.RunningPatience))

			// Assert
			if got != armed.VerdictWaiting {
				t.Errorf("Read(running) %v after arming, an hour after %s = %v, want it waiting",
					armed.RunningPatience, name, got)
			}
		})
	}
}

func TestCIWatchKeepsCountingRunningPatienceAcrossAnOutage(t *testing.T) {
	t.Parallel()

	// Arrange
	// CI reads the same either side of an hour of reads that failed.
	watch, _ := armed.WatchFrom(armedAt()).Read(runningCI(1), armedAt())
	watch, _ = watch.Unheard().Read(runningCI(1), armedAt().Add(time.Hour))

	// Act
	_, got := watch.Read(runningCI(1), armedAt().Add(armed.RunningPatience))

	// Assert
	if got != armed.VerdictRunningTooLong {
		t.Errorf("Read(running) unchanged for %v around an outage = %v, want it running too long",
			armed.RunningPatience, got)
	}
}

func TestStillArmed(t *testing.T) {
	t.Parallel()

	merged := openPull()
	merged.State = forge.StateMerged
	closed := openPull()
	closed.State = forge.StateClosed
	merging := openPull()
	merging.State = forge.StateMerging
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
		"merging first":              {pull: merging, found: true, want: armed.ErrMergingFirst},
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
