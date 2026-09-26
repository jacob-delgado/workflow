// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestARunOffersOnlyPlacesThatResolveToAFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// The second place names a file that resolves to nothing, the way go test
	// prints one relative to its package; offering it would open an empty buffer.
	failing := failingLint()
	failing.unresolved = []string{"b.go"}

	// Act
	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// Assert
	requireScreen(t, failed.View().Content, "a.go:1 first")
	refuseScreen(t, failed.View().Content, "b.go")
}

func TestARunWithNoResolveSeamOffersEveryPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	// With no way to resolve places, none is dropped: better to offer a jump
	// that might miss than to hide every place behind a missing seam.
	deps := failingLint().deps()
	deps.Editor.Resolve = nil
	model := sized(t, tui.New(completeConfig(), nil, deps), 120, 40)

	// Act
	failed := typing(t, drain(t, model, model.Init()), commitKeys("x")...)

	// Assert
	requireScreen(t, failed.View().Content, "a.go:1 first", "b.go:2 second")
}

func TestAFailedRunFindsItsPlacesOffTheUpdateLoop(t *testing.T) {
	t.Parallel()

	// Arrange
	// Finding a place a tool printed relative to its package can walk the whole
	// checkout, which must not freeze the interface while it happens.
	failing := failingLint()
	running, next := pressed(t, typing(t, failing.live(t, 120, 40), "3"), "h")

	// The start, then each line of output, arrives as a message of its own.
	for range len(failing.commitLines) + 1 {
		running, next = finish(t, running, next)
	}

	// Act: deliver the run's end
	failed, finding := finish(t, running, next)

	// Assert: the run shows as failed, with nothing resolved on the update loop
	requireScreen(t, failed.View().Content, "the pre-commit hook failed")

	if calls := failing.asked("resolve"); len(calls) != 0 || finding == nil {
		t.Fatalf("the run's end resolved %q itself, returning command %v; want none, and a command", calls, finding)
	}

	// Act: run the command it returned
	found := finding()

	// Assert: that command resolved the places, all in one call
	if calls := failing.asked("resolve"); len(calls) != 1 {
		t.Errorf("the command resolved the places in calls %q, want one", calls)
	}

	// Act: deliver what it found
	offered, _ := failed.Update(found)

	// Assert: the places are offered to jump to
	requireScreen(t, concrete(t, offered).View().Content, "a.go:1 first", "b.go:2 second")
}

func TestAFailedRunDropsThePlacesOfARunNoLongerShown(t *testing.T) {
	t.Parallel()

	// Arrange
	// A walk of the checkout can outlast the run: the user retries before its
	// places arrive, and the new run must not show the old run's places.
	failing := failingLint()
	running, next := pressed(t, typing(t, failing.live(t, 120, 40), "3"), "h")

	for range len(failing.commitLines) + 1 {
		running, next = finish(t, running, next)
	}

	failed, finding := finish(t, running, next)
	retried, _ := pressed(t, failed, "r")

	// Act
	offered, _ := retried.Update(finding())

	// Assert
	refuseScreen(t, concrete(t, offered).View().Content, "a.go:1 first")
}

func TestAFailedRunOffersAPlacePrintedBeforeTheTailItKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	// A chatty hook names its one place first, then prints far more lines than
	// a run keeps for its tail before it fails.
	failing := failingLint()
	failing.commitLines = slices.Concat(
		[]string{lintJobStarts, firstFailureLine}, slices.Repeat([]string{"compiling"}, 10000),
	)

	// Act
	failed := typing(t, failing.live(t, 120, 40), "3", "h")

	// Assert
	requireScreen(t, failed.View().Content, "the pre-commit hook failed", "a.go:1 first")
}
