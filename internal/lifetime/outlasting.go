// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime

import (
	"context"
	"sync"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// Outliving is how a write's context outlives its caller's: the context to
// write under, with ctx's values and not its end, for the write name says,
// and the function that releases it once the write is done.
type Outliving func(ctx context.Context, name string) (context.Context, context.CancelFunc)

// Unending is a write's context never ending, with nothing to release: for
// writes nothing has yet given an end.
func Unending(ctx context.Context, _ string) (context.Context, context.CancelFunc) {
	return context.WithoutCancel(ctx), func() {}
}

// EndingWith is a write's context ending when program does, the write counted
// in flight, by its name, in the InFlight program carries (CountedIn) until it
// is released, so the process can wait for it before it exits.
func EndingWith(program context.Context) Outliving {
	inFlight := CountedIn(program)

	return func(ctx context.Context, name string) (context.Context, context.CancelFunc) {
		landed := inFlight.Begin(name)
		outlives, release := Until(ctx, program)

		return outlives, func() {
			release()
			landed()
		}
	}
}

// UncountedEndingWith is a context ending when program does, for work counted
// nowhere, which the process does not wait for before it exits: an opener may
// run the browser in the foreground until the browser closes.
func UncountedEndingWith(program context.Context) Outliving {
	return func(ctx context.Context, _ string) (context.Context, context.CancelFunc) {
		return Until(ctx, program)
	}
}

// The Outlasting functions below are a write of one shape, named by name,
// asked under a context outliving its caller's, released once the write
// returns: each is named for how many arguments follow the context, the
// Answer ones for a write that answers a value beside its error, and the Run
// ones for a program whose output streams on after it returns, which holds
// its context, and counts in flight, until it exits, whether or not the
// caller that asked still reads it. A nil write stays nil.

// Outlasting0 is write, named name, asked under the context outlive makes from
// its caller's and released once write returns. A nil write stays nil.
func Outlasting0(outlive Outliving, name string, write func(context.Context) error) func(context.Context) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives)
	}
}

// Outlasting1 is Outlasting0 for a write taking one argument after its
// context.
func Outlasting1[A any](
	outlive Outliving, name string, write func(context.Context, A) error,
) func(context.Context, A) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first)
	}
}

// Outlasting2 is Outlasting0 for a write taking two arguments after its
// context.
func Outlasting2[A, B any](
	outlive Outliving, name string, write func(context.Context, A, B) error,
) func(context.Context, A, B) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second)
	}
}

// Outlasting3 is Outlasting0 for a write taking three arguments after its
// context.
func Outlasting3[A, B, C any](
	outlive Outliving, name string, write func(context.Context, A, B, C) error,
) func(context.Context, A, B, C) error {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) error {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second, third)
	}
}

// OutlastingAnswer0 is Outlasting0 for a write that answers a value beside
// its error.
func OutlastingAnswer0[R any](
	outlive Outliving, name string, write func(context.Context) (R, error),
) func(context.Context) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives)
	}
}

// OutlastingAnswer1 is Outlasting1 for a write that answers a value beside
// its error.
func OutlastingAnswer1[A, R any](
	outlive Outliving, name string, write func(context.Context, A) (R, error),
) func(context.Context, A) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first)
	}
}

// OutlastingAnswer2 is Outlasting2 for a write that answers a value beside
// its error.
func OutlastingAnswer2[A, B, R any](
	outlive Outliving, name string, write func(context.Context, A, B) (R, error),
) func(context.Context, A, B) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second)
	}
}

// OutlastingAnswer3 is Outlasting3 for a write that answers a value beside
// its error.
func OutlastingAnswer3[A, B, C, R any](
	outlive Outliving, name string, write func(context.Context, A, B, C) (R, error),
) func(context.Context, A, B, C) (R, error) {
	if write == nil {
		return nil
	}

	return func(ctx context.Context, first A, second B, third C) (R, error) {
		outlives, release := outlive(ctx, name)
		defer release()

		return write(outlives, first, second, third)
	}
}

// OutlastingRun0 is run, a program whose output streams on after run returns,
// named name and asked under the context outlive makes from its caller's,
// which is released, and stops counting in flight, once Wait has said how the
// program exited. Once its caller's context ends first, the rest of the
// output is read and dropped, so the program never blocks on a full pipe. A
// nil run stays nil.
func OutlastingRun0(
	outlive Outliving, name string, run func(context.Context) (proc.Output, error),
) func(context.Context) (proc.Output, error) {
	if run == nil {
		return nil
	}

	return func(ctx context.Context) (proc.Output, error) {
		outlives, release := outlive(ctx, name)
		output, err := run(outlives)

		return releasedOnExit(ctx, output, err, release)
	}
}

// OutlastingRun1 is OutlastingRun0 for a run taking one argument after its
// context.
func OutlastingRun1[A any](
	outlive Outliving, name string, run func(context.Context, A) (proc.Output, error),
) func(context.Context, A) (proc.Output, error) {
	if run == nil {
		return nil
	}

	return func(ctx context.Context, first A) (proc.Output, error) {
		outlives, release := outlive(ctx, name)
		output, err := run(outlives, first)

		return releasedOnExit(ctx, output, err, release)
	}
}

// releasedOnExit is a run's output releasing the context it was started
// under once Wait has said how its program exited, or at once when it never
// started: releasing it before would kill the program. Once asker, the
// context that asked, ends before Wait is called, as a quit or a switch ends
// the terminal's session mid-run, the asker reads no more of the output, so
// it is read to its end and dropped, and the program waited for: left unread,
// it would block on a full pipe, and its write count in flight until the
// program's lifetime ended. What it printed and how it exited reach no one
// then.
func releasedOnExit(
	asker context.Context, output proc.Output, err error, release context.CancelFunc,
) (proc.Output, error) {
	if err != nil {
		release()

		return output, err
	}

	wait := output.Wait
	output.Wait = sync.OnceValue(func() error {
		defer release()

		return wait()
	})

	left := output
	stopReading := context.AfterFunc(asker, func() { _, _ = left.Drain() })
	output.Wait = func() error {
		stopReading()

		return left.Wait()
	}

	return output, nil
}
