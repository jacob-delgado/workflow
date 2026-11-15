// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package send_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/send"
)

var (
	errRefused = errors.New("the service refused the write")
	errAgain   = errors.New("the service refused it again")
)

// idle is a State that has sent nothing.
func idle(*testing.T) send.State {
	return send.State{}
}

// inFlight is a State with a write out.
func inFlight(t *testing.T) send.State {
	t.Helper()

	state, sent := idle(t).Sent()
	if !sent {
		t.Fatal("an idle State did not send")
	}

	return state
}

// refused is a State whose write was refused with errRefused.
func refused(t *testing.T) send.State {
	t.Helper()

	state, answer := inFlight(t).Answered(errRefused)
	if answer != send.AnswerRefused {
		t.Fatalf("a failed answer was %d; want AnswerRefused", answer)
	}

	return state
}

// view is everything an overlay can see of a State.
type view struct {
	stage  send.Stage
	reason error
}

func viewOf(state send.State) view {
	return view{stage: state.Stage(), reason: state.Reason()}
}
