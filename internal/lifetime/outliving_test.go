// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/lifetime"
)

func TestUnendingGoesOnWhenItsAskerLeavesAndKeepsItsValues(t *testing.T) {
	t.Parallel()

	// Arrange
	asker, leave := context.WithCancel(context.WithValue(t.Context(), valueKey{}, keptValue))
	unending, release := lifetime.Unending(asker, pushing)
	t.Cleanup(release)

	// Act
	leave()

	// Assert
	left, err, value := asker.Err(), unending.Err(), unending.Value(valueKey{})
	if left == nil || err != nil || value != keptValue {
		t.Errorf("the asker ended with %v and the context with %v, keeping %v; want the asker ended and the context "+
			"going on with kept", left, err, value)
	}
}

func TestEndingWithCountsAWriteInFlightByNameUntilItIsReleased(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(0)
	outlive := lifetime.EndingWith(lifetime.Counting(t.Context(), inFlight))

	// Act: begin the write
	outlives, release := outlive(t.Context(), pushing)

	// Assert: it is in flight by its name
	if names := inFlight.Names(); len(names) != 1 || names[0] != pushing {
		t.Fatalf("in flight while the write runs: %q, want [%q]", names, pushing)
	}

	// Act: release it
	release()

	// Assert: nothing is in flight, and the write's context has ended
	if names := inFlight.Names(); len(names) != 0 {
		t.Errorf("in flight once the write was released: %q, want nothing", names)
	}

	if outlives.Err() == nil {
		t.Error("the write's context was still live once the write was released, want it ended")
	}
}

func TestUncountedEndingWithCountsNothingInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(0)
	outlive := lifetime.UncountedEndingWith(lifetime.Counting(t.Context(), inFlight))
	asker := context.WithValue(t.Context(), valueKey{}, keptValue)

	// Act
	outlives, release := outlive(asker, pushing)
	t.Cleanup(release)

	// Assert
	names, value := inFlight.Names(), outlives.Value(valueKey{})
	if len(names) != 0 || value != keptValue {
		t.Errorf("while the uncounted work runs under a context keeping %v, in flight: %q; want it made from its "+
			"asker's, keeping kept, and nothing in flight", value, names)
	}
}

func TestEndingWithEndsTheWriteWithTheProgramsCauseWhenTheProgramEnds(t *testing.T) {
	t.Parallel()

	cases := map[string]func(program context.Context) lifetime.Outliving{
		"counted":   lifetime.EndingWith,
		"uncounted": lifetime.UncountedEndingWith,
	}

	for name, endingWith := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			program, stop := context.WithCancelCause(t.Context())
			outlives, release := endingWith(program)(t.Context(), pushing)
			t.Cleanup(release)

			// Act
			stop(errStopped)

			// Assert
			select {
			case <-outlives.Done():
			case <-time.After(goroutinePatience):
				t.Fatalf("the write's context did not end within %v of its program's", goroutinePatience)
			}

			cause := context.Cause(outlives)
			if !errors.Is(cause, errStopped) {
				t.Errorf("the write's context ended with %v, want the program's cause, %v", cause, errStopped)
			}
		})
	}
}
