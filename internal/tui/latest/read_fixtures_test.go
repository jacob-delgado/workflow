// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package latest_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

var (
	errRead  = errors.New("the read failed")
	errAgain = errors.New("the read failed again")
)

// text is the read these tests keep: of a string, so a value reads plainly.
type text = latest.Read[string]

// view is everything a pane can see of a read.
type view struct {
	stage      latest.Stage
	value      string
	held       bool
	err        error
	out        bool
	refreshing bool
}

func viewOf(read text) view {
	value, held := read.Value()

	return view{
		stage: read.Stage(), value: value, held: held, err: read.Err(), out: read.Out(), refreshing: read.Refreshing(),
	}
}

// nth names a round by the Begin that started it, counting from 1, so a
// table never spells a round's number.
type nth int

const (
	// initRound is round 0, the read Init starts before any Begin.
	initRound nth = 0
	// nowRound is the read's Round as it stands: the round of the latest
	// Begin, Told or Cleared.
	nowRound nth = -1
	// nextRound is the round after the read's Round, which nothing began:
	// what another Read's round can be when a pane passes it to this one.
	nextRound nth = -2
)

// move is what one step of a script does to a read.
type move int

const (
	moveBegin move = iota
	moveAnswer
	moveSettle
	moveTell
	moveClear
	moveSeed
)

// told is the value a script tells a read, and cached the value it seeds
// one with.
const (
	told   = "told"
	cached = "cached"
)

// step is one move of a script a read is played through: an answer, a
// settling or a seed names its round, and an answer carries its value and
// error.
type step struct {
	move  move
	round nth
	value string
	err   error
}

func begin() step { return step{move: moveBegin} }

func answer(round nth, value string, err error) step {
	return step{move: moveAnswer, round: round, value: value, err: err}
}

func settle(round nth) step { return step{move: moveSettle, round: round} }

func tell() step { return step{move: moveTell} }

func clearRead() step { return step{move: moveClear} }

func seed(round nth) step { return step{move: moveSeed, round: round} }

// script is a read and the rounds its Begins started, in order.
type script struct {
	read  text
	begun []latest.Round
}

// played is the zero read played through steps. An answer, a settling or a
// seed the read does not take fails the test: a script arranges only what it means to.
func played(t *testing.T, steps ...step) script {
	t.Helper()

	var scripted script
	for _, next := range steps {
		scripted = scripted.played(t, next)
	}

	return scripted
}

func (scripted script) played(t *testing.T, next step) script {
	t.Helper()

	taken := true

	switch next.move {
	case moveBegin:
		var round latest.Round

		scripted.read, round = scripted.read.Begin()
		scripted.begun = append(scripted.begun, round)
	case moveAnswer:
		scripted.read, taken = scripted.read.Answered(scripted.round(next.round), next.value, next.err)
	case moveSettle:
		scripted.read, taken = scripted.read.Settled(scripted.round(next.round))
	case moveTell:
		scripted.read = scripted.read.Told(told)
	case moveClear:
		scripted.read = scripted.read.Cleared()
	case moveSeed:
		scripted.read, taken = scripted.read.Seeded(scripted.round(next.round), cached)
	}

	if !taken {
		t.Fatalf("arranging, step %+v was not taken by %+v", next, viewOf(scripted.read))
	}

	return scripted
}

// round is the round named: 0, the read's own, the one after it, or the nth
// Begin's.
func (scripted script) round(named nth) latest.Round {
	switch named {
	case initRound:
		return 0
	case nowRound:
		return scripted.read.Round()
	case nextRound:
		return scripted.read.Round() + 1
	default:
		return scripted.begun[named-1]
	}
}
