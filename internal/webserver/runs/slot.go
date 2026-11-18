// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs

import (
	"slices"
	"sync"

	"github.com/jacob-delgado/workflow/internal/api"
)

// runSlot is the one git run going, if any: POST /api/runs claims it before it
// starts a program and frees it once the run has said how it ended, unless
// the next run has taken it by then. Its lock is its own, since the stream
// reads it while the run writes to it.
type runSlot struct {
	mu    sync.Mutex
	state runState
}

// runStage is where the slot's run stands, from claimed to ended.
type runStage int

const (
	// runIdle is no run: none claimed, or the last one freed.
	runIdle runStage = iota
	// runClaimed is a run claimed whose program has not started, so there is
	// nothing to stop yet.
	runClaimed
	// runStopBeforeStart is a claimed run asked to stop before its program
	// started, which stops it the moment it does.
	runStopBeforeStart
	// runGoing is a run whose program is going.
	runGoing
	// runStopping is a run whose program was asked to stop.
	runStopping
	// runEnded is a run that has ended and holds the slot until it has said
	// so, or the next run takes the slot from it.
	runEnded
)

// runState is the slot's run: its stage, the number of the claim that holds
// the slot, the run as the stream shows it, and how to stop its program once
// it has started.
type runState struct {
	stage runStage
	claim int
	run   api.Run
	stop  func()
}

// stopAnswer is what a stop asked of a run makes of it: the run after, what
// stops its program, and whether a run was going to stop.
type stopAnswer struct {
	next  runState
	stops func()
	going bool
}

// claimed is the slot claimed by planned, and the number of that claim, from
// a slot with no run or with one that has ended. Any other stage, a run
// claimed or going, is refused: the slot is answered unchanged, and false.
func (s runState) claimed(planned plannedRun) (runState, int, bool) {
	if s.stage != runIdle && s.stage != runEnded {
		return s, 0, false
	}

	claim := s.claim + 1

	return runState{stage: runClaimed, claim: claim, run: api.Run{
		Kind: planned.kind, Title: planned.title, State: api.RunStateInProgress, Lines: []string{}, Outcome: "",
	}}, claim, true
}

// started records how to stop the run's program once it has started, and
// says whether to stop it now: it was asked to stop before it started. Only a
// claimed run, runClaimed or runStopBeforeStart, is started; any but the
// second goes on.
func (s runState) started(stop func()) (runState, bool) {
	stopNow := s.stage == runStopBeforeStart

	s.stop, s.stage = stop, runGoing
	if stopNow {
		s.stage = runStopping
	}

	return s, stopNow
}

// wrote records a line the run's program wrote. Only a run started, going or
// stopping, writes one, and no stage is read.
func (s runState) wrote(line string) runState {
	s.run.Lines = append(s.run.Lines, line)

	return s
}

// ended records how the run ended — stopped, refused or passed — and answers
// it whole. Only a run started, going or stopping, ends. A run asked to stop
// was stopped only when its program did not pass: a stop kills it, which no
// program passes through, so one that passed had ended before the stop
// reached it; any stage but runStopping reads its exit alone.
func (s runState) ended(planned plannedRun, exit error) (runState, api.Run) {
	switch {
	case s.stage == runStopping && exit != nil:
		s.run.State, s.run.Outcome = api.RunStateStopped, "Stopped "+planned.title+"."
	case exit != nil:
		s.run.State, s.run.Outcome = api.RunStateRefused, planned.refused
	default:
		s.run.State, s.run.Outcome = api.RunStateSucceeded, planned.succeeded
	}

	s.stage = runEnded

	return s, s.run
}

// stopAsked is the run asked to stop, what stops its program now, and
// whether a run was going to stop: none is, with no run or one that has
// ended. A run claimed whose program has not started is stopped the moment it
// does; one already asked to stop is asked again.
func (s runState) stopAsked() (runState, func(), bool) {
	beforeStart, stopping := s, s
	beforeStart.stage, stopping.stage = runStopBeforeStart, runStopping

	answers := map[runStage]stopAnswer{
		runIdle:            {next: s, stops: stopNothing, going: false},
		runClaimed:         {next: beforeStart, stops: stopNothing, going: true},
		runStopBeforeStart: {next: s, stops: stopNothing, going: true},
		runGoing:           {next: stopping, stops: s.stop, going: true},
		runStopping:        {next: s, stops: s.stop, going: true},
		runEnded:           {next: s, stops: stopNothing, going: false},
	}
	answer := answers[s.stage]

	return answer.next, answer.stops, answer.going
}

// stopNothing is the stop of a run whose program has not started, or that
// has ended.
func stopNothing() {}

// freed lets the next run start, unless claim, the run ending, has had the
// slot taken from it already: only the claim that holds the slot frees it.
func (s runState) freed(claim int) runState {
	if s.claim != claim {
		return s
	}

	return runState{stage: runIdle, claim: s.claim, run: api.Run{}, stop: nil}
}

// shown is the run, with its last runTail lines, for a frame of the event
// stream, or nil with none.
func (s runState) shown() *api.Run {
	tail := s.run
	tail.Lines = slices.Clone(tail.Lines[max(0, len(tail.Lines)-runTail):])

	shown := map[runStage]*api.Run{
		runIdle: nil, runClaimed: &tail, runStopBeforeStart: &tail, runGoing: &tail, runStopping: &tail,
		runEnded: &tail,
	}

	return shown[s.stage]
}
