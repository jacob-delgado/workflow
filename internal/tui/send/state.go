// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package send holds the one write an overlay has sent: whether it is out,
// and the reason the service gave when it refused it. An answer changes it
// only while its write is out, so an answer can neither land twice nor land
// where nothing was sent.
package send

// Stage is where an overlay's write stands.
type Stage int

const (
	// StageIdle has no write out and no refusal held. It is the zero stage.
	StageIdle Stage = iota
	// StageInFlight has a write out, whose answer it waits for.
	StageInFlight
	// StageRefused holds the reason the service refused the last write.
	StageRefused
)

// Answer is what an answer did to a State.
type Answer int

const (
	// AnswerLate changed nothing: no write was out to take it.
	AnswerLate Answer = iota
	// AnswerDone took a success: the write went through.
	AnswerDone
	// AnswerRefused took a failure, now held as the refusal.
	AnswerRefused
)

// State is one overlay's write: idle, out, or refused with a reason. The
// zero State is idle.
type State struct {
	stage  Stage
	reason error
}

// Stage is where the write stands.
func (s State) Stage() Stage {
	return s.stage
}

// InFlight reports whether a write is out, waiting for its answer.
func (s State) InFlight() bool {
	return s.stage == StageInFlight
}

// Reason is why the service refused the last write; nil unless Refused.
func (s State) Reason() error {
	return s.reason
}

// Sent puts a write out from Idle or Refused, dropping the refusal, and
// reports true. While a write is out it changes nothing and reports false:
// an overlay keeps one write out at a time.
func (s State) Sent() (State, bool) {
	if s.InFlight() {
		return s, false
	}

	return State{stage: StageInFlight, reason: nil}, true
}

// Answered takes err as the answer to the write out: Idle and AnswerDone on
// a success, Refused with err and AnswerRefused on a failure. Idle and
// Refused have no write out, so either takes no answer: it is unchanged and
// AnswerLate.
func (s State) Answered(err error) (State, Answer) {
	if !s.InFlight() {
		return s, AnswerLate
	}

	if err != nil {
		return State{stage: StageRefused, reason: err}, AnswerRefused
	}

	return State{stage: StageIdle, reason: nil}, AnswerDone
}

// Cleared drops a refusal, back to Idle, where an overlay stops showing it.
// Idle and InFlight are unchanged: a write out still waits for its answer.
func (s State) Cleared() State {
	if s.stage != StageRefused {
		return s
	}

	return State{stage: StageIdle, reason: nil}
}
