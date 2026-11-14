// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package latest keeps what a read of the outside world last answered, so a
// pane shows the answer of the read it began last and no other.
package latest

// Stage is where a read stands, as a pane draws it.
type Stage int

const (
	// StageNeverRead has taken no answer and has no read out. It is the zero
	// stage.
	StageNeverRead Stage = iota
	// StageReading has a read out and nothing held.
	StageReading
	// StageShown took a successful answer last, or was told a value.
	StageShown
	// StageFailed took a failed answer last; an earlier value may still be
	// held.
	StageFailed
)

// Round names one read begun, or one value told or cleared. A later one
// supersedes it. Round 0 is the read a pane starts before it can begin one,
// as Init does.
type Round int

// Read is what a pane last learned from one read of the outside world: the
// value held, the error of the latest answer if it failed, and the round
// whose answer it waits for. The zero Read was never read, and takes the
// answer of round 0 until it begins, is told or is cleared.
type Read[T any] struct {
	stage Stage
	// round is the latest round begun, told or cleared.
	round Round
	// closed is set once round takes no answer: answered, settled, told or
	// cleared. The zero Read leaves round 0 open, for the read Init starts.
	closed bool
	value  T
	held   bool
	err    error
}

// Begin starts a read in a new round, superseding any read still out,
// round 0's included. With nothing held, it is Reading; with a value or an
// error held, it keeps its stage and is Refreshing until the round answers.
func (r Read[T]) Begin() (Read[T], Round) {
	if r.stage == StageNeverRead {
		r.stage = StageReading
	}

	r.round++
	r.closed = false

	return r, r.round
}

// Answered records value and err as round's answer and reports whether it
// was taken. It is taken for the round out, or for round 0 on a Read never
// begun, told or cleared; any other round, and a second answer to a round
// already answered, leaves r unchanged. A success holds value and clears the
// error (Shown). A failure keeps the value already held, holds err and
// ignores the value passed (Failed). Nothing is out afterward.
func (r Read[T]) Answered(round Round, value T, err error) (Read[T], bool) {
	if !r.awaits(round) {
		return r, false
	}

	r.closed = true

	if err != nil {
		r.stage, r.err = StageFailed, err

		return r, true
	}

	r.stage, r.value, r.held, r.err = StageShown, value, true, nil

	return r, true
}

// Settled ends round as answered with nothing new, for an answer a pane
// consumes without showing it (CI's answer held back for a re-run), and
// reports whether it was taken, for the same rounds Answered takes. The
// value, the error and a Shown or Failed stage stay; a first read settled
// holds nothing and is out no longer, so it is NeverRead again. Any other
// round leaves r unchanged.
func (r Read[T]) Settled(round Round) (Read[T], bool) {
	if !r.awaits(round) {
		return r, false
	}

	r.closed = true

	if r.stage == StageReading {
		r.stage = StageNeverRead
	}

	return r, true
}

// Told holds value, learned without a read (a write's own answer), as Shown
// in a new round, so every read still out is superseded.
func (r Read[T]) Told(value T) Read[T] {
	return Read[T]{stage: StageShown, round: r.round + 1, closed: true, value: value, held: true}
}

// Cleared is r never read, in a new round, for a key that changed: an answer
// for the old key is dropped. The round goes on, never back to 0.
func (r Read[T]) Cleared() Read[T] {
	return Read[T]{round: r.round + 1, closed: true}
}

// Seeded holds value as a stand-in for round's answer, a value kept from
// before (a list cached in an earlier session) shown until the answer comes,
// and reports whether it was taken. It is taken for the current round with
// nothing held: still out, a first read then Shown and still Refreshing until
// its answer, which replaces the value; or failed, staying Failed with the
// value now held beside the error. Any other round, a Read never read (round
// 0 before Init's answer, which nothing here began to stand in for, a first
// read settled, or a Read cleared), and a Read holding a value already leave
// r unchanged.
func (r Read[T]) Seeded(round Round, value T) (Read[T], bool) {
	if round != r.round || r.held || r.stage == StageNeverRead {
		return r, false
	}

	if r.stage == StageReading {
		r.stage = StageShown
	}

	r.value, r.held = value, true

	return r, true
}

// WithValue is r holding value, for a change a pane makes to what it shows
// (a further page of a list appended). It neither begins nor answers a round:
// the stage, the error and any round out stay as they are.
func (r Read[T]) WithValue(value T) Read[T] {
	r.value, r.held = value, true

	return r
}

// Stage is where the read stands.
func (r Read[T]) Stage() Stage {
	return r.stage
}

// Value is the value held and whether one is: the latest success's, or the
// latest told, kept across a failure since.
func (r Read[T]) Value() (T, bool) {
	return r.value, r.held
}

// Err is the latest answer's error while the read is Failed, or nil.
func (r Read[T]) Err() error {
	return r.err
}

// Out reports whether a round begun is out: the read is Reading or
// Refreshing. Round 0 is never out, since nothing here began it, so a pane
// that reads in round 0 and refuses a key while its read is out gates on
// Stage as well.
func (r Read[T]) Out() bool {
	return r.round != 0 && !r.closed
}

// Refreshing reports whether a round is out over a value or an error held:
// what a pane's in-flight mark turns for.
func (r Read[T]) Refreshing() bool {
	return r.Out() && r.stage != StageReading
}

// Round is the latest round begun, told or cleared: the round a read begun
// now would supersede.
func (r Read[T]) Round() Round {
	return r.round
}

func (r Read[T]) awaits(round Round) bool {
	return round == r.round && !r.closed
}
