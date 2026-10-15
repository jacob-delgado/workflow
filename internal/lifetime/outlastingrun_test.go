// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/lifetime"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// goroutinePatience is how long a test waits for what happens in a goroutine
// of its own, a run its asker left unread drained and released, or a write's
// context ended once its program's is: long enough never to be what fails a
// test, short enough that a hang fails it.
const goroutinePatience = 10 * time.Second

// run is a streamed write of either shape seen through one signature.
type run func(ctx context.Context) (proc.Output, error)

// runCases are the Run wrappers, each over a run in the one signature above,
// called with the argument its shape takes.
func runCases() map[string]func(lifetime.Outliving, run) run {
	return map[string]func(lifetime.Outliving, run) run{
		"OutlastingRun0": func(outlive lifetime.Outliving, inner run) run {
			return lifetime.OutlastingRun0(outlive, pushing, inner)
		},
		"OutlastingRun1": func(outlive lifetime.Outliving, inner run) run {
			outer := lifetime.OutlastingRun1(outlive, pushing, func(ctx context.Context, _ string) (proc.Output, error) {
				return inner(ctx)
			})

			return func(ctx context.Context) (proc.Output, error) { return outer(ctx, "origin") }
		},
	}
}

// exited is a program's output that has printed lines and exited, its Wait
// counting the calls it was asked.
func exited(waits *atomic.Int32, lines ...string) proc.Output {
	printed := make(chan string, len(lines))
	for _, line := range lines {
		printed <- line
	}

	close(printed)

	return proc.Output{Lines: printed, Wait: func() error {
		waits.Add(1)

		return nil
	}}
}

func TestOutlastingRunReleasesARunThatNeverStartedAtOnce(t *testing.T) {
	t.Parallel()

	for name, wrap := range runCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := newRecording()
			call := wrap(record.outlive, func(context.Context) (proc.Output, error) { return proc.Output{}, errWrite })

			// Act
			_, err := call(t.Context())

			// Assert
			if !errors.Is(err, errWrite) || record.releases() != 1 {
				t.Errorf("a run that never started answered %v and was released %d times, want %v, released once",
					err, record.releases(), errWrite)
			}
		})
	}
}

func TestOutlastingRunHoldsItsContextUntilWaitSaysHowTheProgramExited(t *testing.T) {
	t.Parallel()

	for name, wrap := range runCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := newRecording()

			var (
				waits atomic.Int32
				under any
			)

			call := wrap(record.outlive, func(ctx context.Context) (proc.Output, error) {
				under = ctx.Value(outlivedKey{})

				return exited(&waits), nil
			})

			// Act: start the run
			output, err := call(t.Context())

			// Assert: its context is held while the program runs
			if err != nil || under != pushing || record.releases() != 0 {
				t.Fatalf("the run started with %v under %v, released %d times; want it started under %q's context, "+
					"not yet released", err, under, record.releases(), pushing)
			}

			// Act: wait for the program, twice
			first, second := output.Wait(), output.Wait()

			// Assert: released once, the program waited for once
			if first != nil || second != nil || record.releases() != 1 || waits.Load() != 1 {
				t.Errorf("Wait answered %v, %v, released %d times, waiting %d times; want nil twice, one release "+
					"and one wait", first, second, record.releases(), waits.Load())
			}
		})
	}
}

func TestOutlastingRunDrainsAndReleasesARunItsAskerLeftUnread(t *testing.T) {
	t.Parallel()

	for name, wrap := range runCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			record := newRecording()
			asker, leave := context.WithCancel(t.Context())

			var waits atomic.Int32

			output := exited(&waits, "Enumerating objects: 3", "Writing objects: 100%")
			call := wrap(record.outlive, func(context.Context) (proc.Output, error) { return output, nil })

			_, err := call(asker)
			if err != nil {
				t.Fatalf("the run did not start: %v", err)
			}

			// Act
			leave()

			// Assert
			select {
			case <-record.releasedOnce:
			case <-time.After(goroutinePatience):
				t.Fatalf("a run its asker left unread was not released within %v", goroutinePatience)
			}

			if left, waited := len(output.Lines), waits.Load(); left != 0 || waited != 1 {
				t.Errorf("released with %d lines left unread and the program waited for %d times, want every "+
					"line read and one wait", left, waited)
			}
		})
	}
}

func TestOutlastingRunDrainsNothingOnceItsAskerHasWaited(t *testing.T) {
	t.Parallel()

	for name, wrap := range runCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				asker, leave := context.WithCancel(t.Context())

				var waits atomic.Int32

				output := exited(&waits, "Enumerating objects: 3", "Writing objects: 100%")
				call := wrap(newRecording().outlive, func(context.Context) (proc.Output, error) { return output, nil })

				started, err := call(asker)
				if err != nil {
					t.Fatalf("the run did not start: %v", err)
				}

				err = started.Wait()
				if err != nil {
					t.Fatalf("Wait answered %v, want the program's own nil", err)
				}

				// Act
				leave()

				// Assert
				synctest.Wait()

				left, gone := len(output.Lines), asker.Err()
				if gone == nil || left != 2 {
					t.Errorf("%d lines left unread once an asker that had waited left with %v, want the asker ended "+
						"and both lines left: its Wait stops the drain its leaving would start", left, gone)
				}
			})
		})
	}
}
