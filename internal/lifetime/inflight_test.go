// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/lifetime"
)

// pushing is the work most of these tests hold in flight.
const pushing = "pushing"

// errGaveUp is the cause a wait for work in flight ends with in these tests.
var errGaveUp = errors.New("gave up")

// briefly is a settle short enough not to hold a test up.
const briefly = 10 * time.Millisecond

// forLong is a settle no test waits out.
const forLong = time.Hour

// settledBy is what Settled answered, run on its own, once it answers.
type settledBy struct {
	unfinished []string
	err        error
}

// settling runs Settled under ctx on its own, answering on the channel it
// returns.
func settling(ctx context.Context, inFlight *lifetime.InFlight) <-chan settledBy {
	answered := make(chan settledBy, 1)

	go func() {
		unfinished, err := inFlight.Settled(ctx)
		answered <- settledBy{unfinished: unfinished, err: err}
	}()

	return answered
}

func TestNothingInFlightHasSettled(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)

	// Act
	unfinished, err := inFlight.Settled(t.Context())

	// Assert
	if err != nil || unfinished != nil {
		t.Errorf("Settled = %q, %v; want it settled", unfinished, err)
	}
}

func TestWorkInFlightIsNamedInTheOrderItBegan(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)
	inFlight.Begin(pushing)
	posted := inFlight.Begin("posting a message")
	inFlight.Begin("caching the issue list")

	// Act
	posted()

	// Assert
	want := []string{pushing, "caching the issue list"}
	if got := inFlight.Names(); !slices.Equal(got, want) {
		t.Errorf("Names = %q, want %q", got, want)
	}
}

func TestEndingWorkTwiceEndsItOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)
	pushed := inFlight.Begin(pushing)
	inFlight.Begin(pushing)
	pushed()

	// Act
	pushed()

	// Assert
	if got := inFlight.Names(); !slices.Equal(got, []string{pushing}) {
		t.Errorf("Names = %q, want the second push still in flight", got)
	}
}

func TestSettledWaitsForWorkToEnd(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)
	pushed := inFlight.Begin(pushing)
	answered := settling(t.Context(), inFlight)

	// Act
	pushed()

	// Assert
	select {
	case settled := <-answered:
		if settled.err != nil || settled.unfinished != nil {
			t.Errorf("Settled = %q, %v; want it settled once the push ended", settled.unfinished, settled.err)
		}
	case <-time.After(time.Minute):
		t.Fatal("Settled had not answered once the push ended")
	}
}

// A piece of work that follows another begins just as the first ends, and is
// waited for too, though it begins once the wait has seen nothing in flight.
func TestSettledWaitsForWorkThatBeginsWhileItSettles(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(time.Second)
	inFlight.Begin("posting a message")()

	answered := settling(t.Context(), inFlight)

	time.Sleep(100 * time.Millisecond)

	// Act: the post's record begins while the wait settles
	recorded := inFlight.Begin("remembering an announcement")

	// Assert: the wait goes on for the record, past the settle
	select {
	case settled := <-answered:
		t.Fatalf("Settled = %q, %v while the record was in flight, want it to wait", settled.unfinished, settled.err)
	case <-time.After(1500 * time.Millisecond):
	}

	// Act: the record ends
	recorded()

	// Assert: the wait is over
	select {
	case settled := <-answered:
		if settled.err != nil {
			t.Errorf("Settled = %v, want it settled once the record ended", settled.err)
		}
	case <-time.After(time.Minute):
		t.Fatal("Settled had not answered once the record ended")
	}
}

func TestSettledNamesTheWorkAnEndedWaitLeavesInFlight(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)
	inFlight.Begin(pushing)

	waiting, giveUp := context.WithCancelCause(t.Context())
	giveUp(errGaveUp)

	// Act
	unfinished, err := inFlight.Settled(waiting)

	// Assert
	if !errors.Is(err, errGaveUp) || !slices.Equal(unfinished, []string{pushing}) {
		t.Errorf("Settled = %q, %v; want the push named, and %v", unfinished, err, errGaveUp)
	}
}

// A wait that ends while the last piece of work settles has nothing left to
// give up on.
func TestAWaitEndedWhileSettlingLeavesNothingUnfinished(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(forLong)
	inFlight.Begin(pushing)()

	waiting, giveUp := context.WithTimeoutCause(t.Context(), briefly, errGaveUp)
	t.Cleanup(giveUp)

	// Act
	unfinished, err := inFlight.Settled(waiting)

	// Assert
	if err != nil || unfinished != nil {
		t.Errorf("Settled = %q, %v; want nothing left unfinished", unfinished, err)
	}
}

func TestWorkIsCountedInWhatItsContextCarries(t *testing.T) {
	t.Parallel()

	// Arrange
	inFlight := lifetime.NewInFlight(briefly)
	ctx := lifetime.Counting(t.Context(), inFlight)

	// Act
	lifetime.CountedIn(ctx).Begin(pushing)

	// Assert
	if got := inFlight.Names(); !slices.Equal(got, []string{pushing}) {
		t.Errorf("Names = %q, want the push counted in the InFlight its context carries", got)
	}
}

func TestWorkUnderAContextCarryingNoCountIsCountedNowhereWaitedOn(t *testing.T) {
	t.Parallel()

	// Arrange
	counted := lifetime.CountedIn(t.Context())

	// Act
	counted.Begin(pushing)

	// Assert
	if got := lifetime.CountedIn(t.Context()).Names(); len(got) != 0 {
		t.Errorf("a context carrying no count holds %q, want each ask to answer a count of its own", got)
	}
}
