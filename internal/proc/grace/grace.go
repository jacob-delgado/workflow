// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package grace counts down how long a streamed run's output may hold the run
// once its program has exited. The count runs only while the run waits on the
// pipe, never while it hands a line to a reader, so a slow reader still gets
// every line the program wrote, while a process the program left behind,
// holding the pipe open, ends the run once the grace is spent.
//
// A Watch is a value moved by the run's events, each given the time it came
// at; it holds no timer. Whoever drives it starts a timer for Left whenever
// the stage is StageCounting and drops it otherwise. Each move does something
// in two stages and nothing in the rest, so it compares the stage to those
// two rather than switching on every one: an exhaustive switch's last case
// would only ever be seen true, which the changed-lines coverage gate fails.
package grace

import "time"

// Stage is where a run's output stands against the grace.
type Stage int

const (
	// StageSending has the program running while the run is not waiting on
	// the pipe: handing a line on, or not yet reading. It is where a watch
	// starts.
	StageSending Stage = iota
	// StageReading has the program running while the run waits on the pipe.
	StageReading
	// StagePaused has the program exited while the run hands a line on: the
	// count holds where it was.
	StagePaused
	// StageCounting has the program exited while the run waits on the pipe:
	// the count runs.
	StageCounting
)

// Watch is a run's output against its grace: the stage, what is left of the
// grace, and since when it has been counting, read only in StageCounting.
type Watch struct {
	stage Stage
	left  time.Duration
	since time.Time
}

// Start is a watch over a program just started, with patience to spend once
// it exits.
func Start(patience time.Duration) Watch {
	return Watch{stage: StageSending, left: patience, since: time.Time{}}
}

// Stage is where the watch stands.
func (w Watch) Stage() Stage {
	return w.stage
}

// Left is what remains of the grace at the time given: the grace less the
// time counted, never below nothing.
func (w Watch) Left(now time.Time) time.Duration {
	if w.stage != StageCounting {
		return w.left
	}

	return max(w.left-now.Sub(w.since), 0)
}

// OnPipe is the run beginning to wait on the pipe at the time given: a paused
// count runs again from then. In any other stage it changes nothing, since
// the run is waiting on the pipe already.
func (w Watch) OnPipe(now time.Time) Watch {
	if w.stage == StageSending {
		return Watch{stage: StageReading, left: w.left, since: w.since}
	}

	if w.stage == StagePaused {
		return Watch{stage: StageCounting, left: w.left, since: now}
	}

	return w
}

// OffPipe is the run leaving the pipe at the time given, a line read to hand
// on: a running count pauses, keeping what it had left. In any other stage it
// changes nothing, since the run is off the pipe already.
func (w Watch) OffPipe(now time.Time) Watch {
	if w.stage == StageReading {
		return Watch{stage: StageSending, left: w.left, since: w.since}
	}

	if w.stage == StageCounting {
		return Watch{stage: StagePaused, left: w.Left(now), since: w.since}
	}

	return w
}

// Reaped is the program exiting at the time given: the count starts if the
// run waits on the pipe, and is paused if it is handing a line on. Once the
// program has exited it changes nothing, since a program exits once.
func (w Watch) Reaped(now time.Time) Watch {
	if w.stage == StageReading {
		return Watch{stage: StageCounting, left: w.left, since: now}
	}

	if w.stage == StageSending {
		return Watch{stage: StagePaused, left: w.left, since: w.since}
	}

	return w
}
