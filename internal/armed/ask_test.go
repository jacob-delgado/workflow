// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

// A surface can ask for a read of the hold while another is out, on its beat
// and on a refresh, and the forge answers the two in whatever order it
// likes. The hold hears only the read asked last in its round.

import (
	"reflect"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// answer is a read, asked as ask, stepping hold on what it heard.
type answer func(hold armed.Hold, ask armed.Ask) (armed.Hold, armed.Next)

// passedRead is a read, asked as ask, that heard CI pass on the held pull
// request, after an hour.
func passedRead(hold armed.Hold, ask armed.Ask) (armed.Hold, armed.Next) {
	return hold.Heard(armed.Reading{
		Ask: ask, Pull: heldPull(), Found: true, CI: forge.CI{State: forge.CIPassed}, At: after(time.Hour),
	})
}

// failedRead is a read, asked as ask, that failed with errSeam, after an
// hour.
func failedRead(hold armed.Hold, ask armed.Ask) (armed.Hold, armed.Next) {
	return hold.Unheard(armed.Failure{Ask: ask, Err: errSeam, At: after(time.Hour)})
}

// askedOf is from's hold with a read of it asked, and that read.
func askedOf(from func(*testing.T) armed.Hold) func(*testing.T) (armed.Hold, armed.Ask) {
	return func(t *testing.T) (armed.Hold, armed.Ask) {
		t.Helper()

		return from(t).Asked()
	}
}

// askedBeforeTheLast is a running hold a read was asked of, then another,
// which heard a check fail; and the first, still out.
func askedBeforeTheLast(t *testing.T) (armed.Hold, armed.Ask) {
	t.Helper()

	hold, older := running(t).Asked()
	hold, newer := hold.Asked()
	hold, _ = hold.Heard(armed.Reading{
		Ask: newer, Pull: heldPull(), Found: true, CI: forge.CI{State: forge.CIFailed}, At: after(2 * time.Minute),
	})

	return hold, older
}

// askedFirstOfTwo is a running hold a read was asked of, then another, both
// still out; and the first.
func askedFirstOfTwo(t *testing.T) (armed.Hold, armed.Ask) {
	t.Helper()

	hold, older := running(t).Asked()
	hold, _ = hold.Asked()

	return hold, older
}

// askedTheRoundBefore is a hold armed afresh once the one a read was asked
// of was stopped; and that read, asked the round before.
func askedTheRoundBefore(t *testing.T) (armed.Hold, armed.Ask) {
	t.Helper()

	hold, older := running(t).Asked()
	hold, _ = hold.Stop()

	rearmed, err := hold.Arm(after(2*time.Minute), arming())
	if err != nil {
		t.Fatalf("arming a stopped hold: %v", err)
	}

	return rearmed.Hold, older
}

// askedInNoRound is a running hold, and the zero Ask.
func askedInNoRound(t *testing.T) (armed.Hold, armed.Ask) {
	t.Helper()

	return running(t), armed.Ask{}
}

func TestAReadNotAskedLastWhileTheHoldWaitsChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		asked  func(*testing.T) (armed.Hold, armed.Ask)
		answer answer
	}{
		"a read asked before the last, answering after it":    {asked: askedBeforeTheLast, answer: passedRead},
		"a failure asked before the last, answering after it": {asked: askedBeforeTheLast, answer: failedRead},
		"a read asked before the last, answering first":       {asked: askedFirstOfTwo, answer: passedRead},
		"a failure asked before the last, answering first":    {asked: askedFirstOfTwo, answer: failedRead},
		"a read asked the round before":                       {asked: askedTheRoundBefore, answer: passedRead},
		"a failure asked the round before":                    {asked: askedTheRoundBefore, answer: failedRead},
		"a read asked in no round":                            {asked: askedInNoRound, answer: passedRead},
		"a read once posted":                                  {asked: askedOf(posted), answer: passedRead},
		"a read while posting":                                {asked: askedOf(due), answer: passedRead},
		"a failure once stopped":                              {asked: askedOf(stopped), answer: failedRead},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold, ask := tt.asked(t)

			// Act
			got, next := tt.answer(hold, ask)

			// Assert
			if !reflect.DeepEqual(got, hold) || next != armed.NextNothing {
				t.Errorf("stepped to %s, asking for %s; want it unchanged, asking for nothing",
					stageName(got.Stage()), nextName(next))
			}
		})
	}
}

func TestTheReadAskedLastIsHeardWhileAnEarlierOneIsOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer answer
		stage  armed.Stage
		next   armed.Next
	}{
		"heard":  {answer: passedRead, stage: armed.StageDue, next: armed.NextDeliver},
		"failed": {answer: failedRead, stage: armed.StageUnheard, next: armed.NextReadLater},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold, _ := running(t).Asked()
			hold, newer := hold.Asked()

			// Act
			got, next := tt.answer(hold, newer)

			// Assert
			if got.Stage() != tt.stage || next != tt.next {
				t.Errorf("stepped to %s, asking for %s; want %s, asking for %s",
					stageName(got.Stage()), nextName(next), stageName(tt.stage), nextName(tt.next))
			}
		})
	}
}

func TestAReadOutAsTheHeldAnnouncementIsRetoldIsHeard(t *testing.T) {
	t.Parallel()

	// Arrange
	hold, ask := running(t).Asked()

	retold, err := hold.Arm(after(3*time.Minute), arming())
	if err != nil {
		t.Fatalf("arming the held announcement again: %v", err)
	}

	// Act
	got, next := passedRead(retold.Hold, ask)

	// Assert
	if got.Stage() != armed.StageDue || next != armed.NextDeliver {
		t.Errorf("stepped to %s, asking for %s; want it due, asking for the post",
			stageName(got.Stage()), nextName(next))
	}
}
