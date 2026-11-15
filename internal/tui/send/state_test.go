// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package send_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/send"
)

func TestTheZeroStateIsIdleWithNoReason(t *testing.T) {
	t.Parallel()

	// Arrange
	var state send.State

	// Act
	got := viewOf(state)

	// Assert
	if got != (view{stage: send.StageIdle, reason: nil}) {
		t.Errorf("the zero State is %+v; want idle with no reason", got)
	}
}

func TestSendingWithNoWriteOutPutsOneOut(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*testing.T) send.State{
		"idle": idle, "refused, dropping the reason": refused,
	}

	for name, from := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			state := from(t)

			// Act
			sending, sent := state.Sent()

			// Assert
			if got := viewOf(sending); !sent || got != (view{stage: send.StageInFlight, reason: nil}) {
				t.Errorf("Sent is %+v, %t; want in flight with no reason, true", got, sent)
			}
		})
	}
}

func TestASecondSendWhileAWriteIsOutChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	state := inFlight(t)

	// Act
	again, sent := state.Sent()

	// Assert
	if sent || again != state {
		t.Errorf("Sent is %+v, %t; want %+v unchanged, false", viewOf(again), sent, viewOf(state))
	}
}

func TestTheAnswerToTheWriteOutLands(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err    error
		want   view
		answer send.Answer
	}{
		"a success is done, and idle": {
			err: nil, want: view{stage: send.StageIdle, reason: nil}, answer: send.AnswerDone,
		},
		"a failure is refused, holding why": {
			err: errRefused, want: view{stage: send.StageRefused, reason: errRefused}, answer: send.AnswerRefused,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			state := inFlight(t)

			// Act
			answered, answer := state.Answered(tt.err)

			// Assert
			if got := viewOf(answered); answer != tt.answer || got != tt.want {
				t.Errorf("Answered is %+v, %d; want %+v, %d", got, answer, tt.want, tt.answer)
			}
		})
	}
}

func TestAnAnswerWithNoWriteOutIsLate(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) send.State
		err  error
	}{
		"a success while idle":           {from: idle, err: nil},
		"a failure while idle":           {from: idle, err: errAgain},
		"a success after the refusal":    {from: refused, err: nil},
		"a second failure after refusal": {from: refused, err: errAgain},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			state := tt.from(t)

			// Act
			answered, answer := state.Answered(tt.err)

			// Assert
			if answer != send.AnswerLate || answered != state {
				t.Errorf("Answered is %+v, %d; want %+v unchanged, AnswerLate", viewOf(answered), answer, viewOf(state))
			}
		})
	}
}

func TestClearingDropsOnlyARefusal(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) send.State
		want view
	}{
		"refused, back to idle": {from: refused, want: view{stage: send.StageIdle, reason: nil}},
		"idle, unchanged":       {from: idle, want: view{stage: send.StageIdle, reason: nil}},
		"in flight, still out":  {from: inFlight, want: view{stage: send.StageInFlight, reason: nil}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			state := tt.from(t)

			// Act
			cleared := state.Cleared()

			// Assert
			if got := viewOf(cleared); got != tt.want {
				t.Errorf("Cleared is %+v; want %+v", got, tt.want)
			}
		})
	}
}
