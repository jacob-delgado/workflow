// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// readyPull is #9, ready to merge in every way but its CI, at head.
func readyPull(head string) forge.PullRequest {
	pull := openPull()
	pull.HeadCommit, pull.Approvals, pull.Mergeable = head, 1, forge.MergeClean

	return pull
}

// noChecks is CI that reports no check.
func noChecks() forge.CI {
	return forge.CI{State: forge.CINone}
}

// heardNone is a HeadWatch that heard pull report no check at each moment
// after armedAt in at.
func heardNone(pull forge.PullRequest, at ...time.Duration) armed.HeadWatch {
	var watch armed.HeadWatch

	for _, after := range at {
		watch = watch.Read(pull, noChecks(), armedAt().Add(after))
	}

	return watch
}

func TestHeadWatchOffersTheMergeUnderNoCIOnlyOnceItsHeadWasQuietForThePatience(t *testing.T) {
	t.Parallel()

	pull := readyPull("c0ffee1")

	cases := map[string]struct {
		heard []time.Duration
		want  bool
	}{
		"never read":         {heard: nil, want: false},
		"read once":          {heard: []time.Duration{0}, want: false},
		"a moment too early": {heard: []time.Duration{0, armed.NoCIPatience - time.Second}, want: false},
		"quiet for as long":  {heard: []time.Duration{0, armed.NoCIPatience / 2, armed.NoCIPatience}, want: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			watch := heardNone(pull, tt.heard...)

			// Act
			got := watch.CanMerge(pull, noChecks())

			// Assert
			if got != tt.want {
				t.Errorf("CanMerge after reads at %v = %t, want %t", tt.heard, got, tt.want)
			}
		})
	}
}

func TestHeadWatchStartsAgainOnANewHead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A push to the pull request: its new head's checks are listed a moment
	// after it lands.
	before, pushed := readyPull("c0ffee1"), readyPull("c0ffee2")
	watch := heardNone(before, 0, armed.NoCIPatience).Read(pushed, noChecks(), armedAt().Add(armed.NoCIPatience*2))

	// Act
	got := watch.CanMerge(pushed, noChecks())

	// Assert
	if got || watch.NoCI(before) {
		t.Errorf("CanMerge = %t, NoCI of the old head %t; want neither once the head moved", got, watch.NoCI(before))
	}
}

// A pull request found at a new head before CI is read on it is not offered
// its merge on the quiet its old head was heard to keep.
func TestHeadWatchHoldsTheQuietToTheHeadItWasHeardOn(t *testing.T) {
	t.Parallel()

	// Arrange
	watch := heardNone(readyPull("c0ffee1"), 0, armed.NoCIPatience)
	pushed := readyPull("c0ffee2")

	// Act & Assert
	if watch.NoCI(pushed) || watch.CanMerge(pushed, noChecks()) {
		t.Errorf("HeadWatch heard on c0ffee1 = NoCI %t, CanMerge %t on c0ffee2; want neither",
			watch.NoCI(pushed), watch.CanMerge(pushed, noChecks()))
	}
}

func TestHeadWatchStartsAgainOnACheckReported(t *testing.T) {
	t.Parallel()

	// Arrange
	pull := readyPull("c0ffee1")
	watch := heardNone(pull, 0, armed.NoCIPatience).
		Read(pull, forge.CI{State: forge.CIRunning}, armedAt().Add(armed.NoCIPatience+time.Minute)).
		Read(pull, noChecks(), armedAt().Add(armed.NoCIPatience+2*time.Minute))

	// Act
	got := watch.NoCI(pull)

	// Assert
	if got {
		t.Error("NoCI = true, want the quiet counted again from the check reported")
	}
}

func TestHeadWatchCountsNoQuietAcrossAReadThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	// An outage between two reads that heard none is not counted as quiet.
	pull := readyPull("c0ffee1")
	watch := heardNone(pull, 0).Unheard().Read(pull, noChecks(), armedAt().Add(armed.NoCIPatience))

	// Act
	got := watch.NoCI(pull)

	// Assert
	if got {
		t.Error("NoCI = true, want the outage left out of the quiet")
	}
}

func TestHeadWatchKeepsANoCIItHeardPastAReadThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	pull := readyPull("c0ffee1")
	watch := heardNone(pull, 0, armed.NoCIPatience).Unheard()

	// Act
	got := watch.CanMerge(pull, noChecks())

	// Assert
	if !got {
		t.Error("CanMerge = false, want the quiet already heard kept")
	}
}

func TestHeadWatchTakesCIThatPassedAtOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	pull := readyPull("c0ffee1")

	var watch armed.HeadWatch

	// Act
	got := watch.CanMerge(pull, forge.CI{State: forge.CIPassed})

	// Assert
	if !got {
		t.Error("CanMerge = false, want CI that passed taken without waiting")
	}
}

func TestHeadWatchStillAsksForEverythingElseLoopAsks(t *testing.T) {
	t.Parallel()

	// Arrange
	pull := readyPull("c0ffee1")
	pull.Approvals = 0
	watch := heardNone(pull, 0, armed.NoCIPatience)

	// Act
	got := watch.CanMerge(pull, noChecks())

	// Assert
	if got {
		t.Error("CanMerge = true for a pull request no one approved")
	}
}
