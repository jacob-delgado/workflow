// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// step is one event a hold is stepped on.
type step func(armed.Hold) (armed.Hold, armed.Next)

// stepCase is a row of the hold's transition table: the hold it starts from,
// the event, and the stage, the effect and the reason it comes to. A nil
// reason wants none.
type stepCase struct {
	from   func(*testing.T) armed.Hold
	event  step
	stage  armed.Stage
	next   armed.Next
	reason error
}

// reasonIs reports whether reason is want: none for none, else one that
// wraps it.
func reasonIs(reason, want error) bool {
	if want == nil {
		return reason == nil
	}

	return errors.Is(reason, want)
}

// hearing is the read of a find that answered pull, found or not, its CI at
// state, after elapsed.
func hearing(pull forge.PullRequest, found bool, state forge.CIState, elapsed time.Duration) step {
	return func(hold armed.Hold) (armed.Hold, armed.Next) {
		return hold.Heard(armed.Reading{
			Round: hold.Round(), Pull: pull, Found: found, CI: forge.CI{State: state}, At: after(elapsed),
		})
	}
}

// failing is a read that failed with err after elapsed.
func failing(err error, elapsed time.Duration) step {
	return func(hold armed.Hold) (armed.Hold, armed.Next) {
		return hold.Unheard(armed.Failure{Round: hold.Round(), Err: err, At: after(elapsed)})
	}
}

// delivering is a post whose outcome was err, after elapsed.
func delivering(err error, elapsed time.Duration) step {
	return func(hold armed.Hold) (armed.Hold, armed.Next) {
		return hold.Delivered(armed.Outcome{Round: hold.Round(), Err: err, At: after(elapsed)})
	}
}

// with is pull in state.
func with(pull forge.PullRequest, state forge.PullState) forge.PullRequest {
	pull.State = state

	return pull
}

// heardRows are the rows of a read heard: what the read saw.
func heardRows() map[string]stepCase {
	return map[string]stepCase{
		"listing, not listed yet": {
			from: listing, event: hearing(forge.PullRequest{}, false, forge.CINone, time.Minute),
			stage: armed.StageListing, next: armed.NextReadLater, reason: armed.ErrNotListedYet,
		},
		"listing, found running (P025)": {
			from: listing, event: hearing(heldPull(), true, forge.CIRunning, time.Minute),
			stage: armed.StageRunning, next: armed.NextReadLater,
		},
		"listing, found closed": {
			from: listing, event: hearing(with(heldPull(), forge.StateClosed), true, forge.CINone, time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrPullChanged,
		},
		"found before, now gone": {
			from: running, event: hearing(forge.PullRequest{}, false, forge.CINone, 2*time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrPullChanged,
		},
		"#12 found in its place": {
			from: running, event: hearing(otherPull(), true, forge.CIRunning, 2*time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrPullChanged,
		},
		"merged first": {
			from: awaitingFix, event: hearing(with(heldPull(), forge.StateMerged), true, forge.CINone, 2*time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrMergedFirst,
		},
		"CI running": {
			from: awaitingChecks, event: hearing(heldPull(), true, forge.CIRunning, time.Minute),
			stage: armed.StageRunning, next: armed.NextReadLater,
		},
		"a check failed": {
			from: running, event: hearing(heldPull(), true, forge.CIFailed, 2*time.Minute),
			stage: armed.StageAwaitingFix, next: armed.NextReadLater,
		},
		"CI running again past a failure": {
			from: awaitingFix, event: hearing(heldPull(), true, forge.CIRunning, 2*time.Minute),
			stage: armed.StageRunning, next: armed.NextReadLater,
		},
		"no check reported yet": {
			from: running, event: hearing(heldPull(), true, forge.CINone, 2*time.Minute),
			stage: armed.StageAwaitingChecks, next: armed.NextReadLater,
		},
		"heard again past a failed read": {
			from: unheard, event: hearing(heldPull(), true, forge.CIRunning, 3*time.Minute),
			stage: armed.StageRunning, next: armed.NextReadLater,
		},
		"no check within patience": {
			from: awaitingChecks, event: hearing(heldPull(), true, forge.CINone, armed.NoCIPatience),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrNoCI,
		},
		"CI passed": {
			from: running, event: hearing(heldPull(), true, forge.CIPassed, 2*time.Minute),
			stage: armed.StageDue, next: armed.NextDeliver,
		},
	}
}

// notListedYet is a hold just opened that a read did not find listed.
func notListedYet(t *testing.T) armed.Hold {
	t.Helper()

	hold, _ := hearing(forge.PullRequest{}, false, forge.CINone, 0)(listing(t))

	return hold
}

// unheardRows are the rows of a read that failed, or a pull request just
// opened not listed yet: waited past within the patience.
func unheardRows() map[string]stepCase {
	return map[string]stepCase{
		"while CI runs": {
			from: running, event: failing(errSeam, 2*time.Minute),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: errSeam,
		},
		"awaiting checks": {
			from: awaitingChecks, event: failing(errSeam, time.Minute),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: errSeam,
		},
		"listing stays listing": {
			from: listing, event: failing(errSeam, time.Minute),
			stage: armed.StageListing, next: armed.NextReadLater, reason: errSeam,
		},
		"unheard since a minute (C38)": {
			from: unheard, event: failing(errSeam, 2*time.Minute+armed.UnheardPatience-time.Second),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: errSeam,
		},
		"unheard for its whole patience (C38)": {
			from: unheard, event: failing(errSeam, 2*time.Minute+armed.UnheardPatience),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrUnheard,
		},
		"not listed for its whole patience (C37)": {
			from:  notListedYet,
			event: hearing(forge.PullRequest{}, false, forge.CINone, armed.UnheardPatience),
			stage: armed.StageDropped, next: armed.NextNothing, reason: armed.ErrNotListedYet,
		},
	}
}

func TestAHoldStepsAsItsTransitionTableSays(t *testing.T) {
	t.Parallel()

	rows := heardRows()
	maps.Copy(rows, unheardRows())
	maps.Copy(rows, deliveredRows())

	for name, tt := range rows {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := tt.from(t)

			// Act
			got, next := tt.event(hold)

			// Assert
			if got.Stage() != tt.stage || next != tt.next {
				t.Errorf("stepped to %s, asking for %s (%v); want %s, asking for %s",
					stageName(got.Stage()), nextName(next), got.Reason(), stageName(tt.stage), nextName(tt.next))
			}

			if !reasonIs(got.Reason(), tt.reason) {
				t.Errorf("stepped with reason %v, want %v", got.Reason(), tt.reason)
			}
		})
	}
}

func TestAHoldUnheardForItsPatienceSaysWhyAndWhatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	hold := unheard(t)

	// Act
	got, _ := hold.Unheard(armed.Failure{
		Round: hold.Round(), Err: errSeam, At: after(2*time.Minute + armed.UnheardPatience),
	})

	// Assert
	if !errors.Is(got.Reason(), armed.ErrUnheard) || !errors.Is(got.Reason(), errSeam) {
		t.Errorf("reason = %v, want ErrUnheard wrapping the last failure", got.Reason())
	}
}

func TestAHeardReadStartsTheUnheardPatienceAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// Unheard at 2m, heard at 5m: a failure at 14m is four minutes unheard,
	// not twelve.
	hold := heardAs(unheard(t), heldPull(), forge.CIRunning, 5*time.Minute)
	hold = unheardAt(hold, 6*time.Minute)

	// Act
	got, next := hold.Unheard(armed.Failure{Round: hold.Round(), Err: errSeam, At: after(14 * time.Minute)})

	// Assert
	if got.Stage() != armed.StageUnheard || next != armed.NextReadLater {
		t.Errorf("stepped to %s, asking for %s (%v); want it still waiting past the failure",
			stageName(got.Stage()), nextName(next), got.Reason())
	}
}

func TestAnOutageIsNotCountedAsQuiet(t *testing.T) {
	t.Parallel()

	// Arrange
	// No check heard at 1m, then reads that failed until 10m: quiet is a
	// minute, not ten (P022).
	hold := heardAs(awaitingChecks(t), heldPull(), forge.CINone, time.Minute)
	hold = unheardAt(hold, 2*time.Minute)
	hold = unheardAt(hold, 9*time.Minute)

	// Act
	got, next := hold.Heard(armed.Reading{
		Round: hold.Round(), Pull: heldPull(), Found: true, CI: forge.CI{State: forge.CINone}, At: after(10 * time.Minute),
	})

	// Assert
	if got.Stage() != armed.StageAwaitingChecks || next != armed.NextReadLater {
		t.Errorf("stepped to %s (%v); want it still awaiting checks", stageName(got.Stage()), got.Reason())
	}
}

func TestADueHoldPostsThePullRequestAndHeadItRead(t *testing.T) {
	t.Parallel()

	// Arrange
	read := heldPull()
	read.Title, read.HeadCommit = "Redact the token, retitled", "f00d"

	// Act
	got, _ := hearing(read, true, forge.CIPassed, 2*time.Minute)(running(t))

	// Assert
	// P023: the head CI passed at is the forge's, never the checkout's.
	if delivery := got.Delivery(); delivery.Pull != read || delivery.Head != "f00d" {
		t.Errorf("delivery = %+v at %q, want the pull request read, at the head it read", delivery.Pull, delivery.Head)
	}
}

// lintCheck is the check the gate saw fail.
const lintCheck = "lint"

// refusedFor is the gate's refusal for a check that failed, ci's.
func refusedFor(ci forge.CI) error {
	return loop.GateRefusalError{CI: ci, Err: loop.ErrCIFailed}
}

// deliveredRows are the rows of a post's outcome: how the post went.
func deliveredRows() map[string]stepCase {
	failedCheck := forge.CI{State: forge.CIFailed, Checks: []forge.Check{{Name: lintCheck, State: forge.CIFailed}}}
	rateLimited := fmt.Errorf("%w: %w", loop.ErrNotDelivered, httpx.ErrRateLimited)
	refused := fmt.Errorf("%w: %w", loop.ErrNotDelivered, messaging.ErrPostRefused)

	return map[string]stepCase{
		"delivered": {
			from: due, event: delivering(nil, 2*time.Minute), stage: armed.StagePosted, next: armed.NextNothing,
		},
		"posted, not confirmed": {
			from: due, event: delivering(fmt.Errorf("%w: busy", loop.ErrUnconfirmed), 2*time.Minute),
			stage: armed.StagePosted, next: armed.NextNothing, reason: loop.ErrUnconfirmed,
		},
		"a check failed by the gate's read (C39)": {
			from: due, event: delivering(refusedFor(failedCheck), 2*time.Minute),
			stage: armed.StageAwaitingFix, next: armed.NextReadLater, reason: loop.ErrCIFailed,
		},
		"the gate could not read CI": {
			from: due, event: delivering(loop.GateRefusalError{Err: loop.ErrCIUnread}, 2*time.Minute),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: loop.ErrCIUnread,
		},
		"the store could not claim it": {
			from: due, event: delivering(fmt.Errorf("%w: busy", loop.ErrNotClaimed), 2*time.Minute),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: loop.ErrNotClaimed,
		},
		"rate limited": {
			from: due, event: delivering(rateLimited, 2*time.Minute),
			stage: armed.StageUnheard, next: armed.NextReadLater, reason: httpx.ErrRateLimited,
		},
		"claimed elsewhere": {
			from: due, event: delivering(loop.AnnouncedAlreadyError{}, 2*time.Minute),
			stage: armed.StageElsewhere, next: armed.NextNothing, reason: loop.ErrAnnouncedAlready,
		},
		"refused by the service": {
			from: due, event: delivering(refused, 2*time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: loop.ErrNotDelivered,
		},
		"may have gone out": {
			from: due, event: delivering(fmt.Errorf("%w: %w", loop.ErrPostUncertain, messaging.ErrUnreachable), 0),
			stage: armed.StageDropped, next: armed.NextNothing, reason: loop.ErrPostUncertain,
		},
		"nothing to post with": {
			from: due, event: delivering(loop.ErrAnnounceUnavailable, 2*time.Minute),
			stage: armed.StageDropped, next: armed.NextNothing, reason: loop.ErrAnnounceUnavailable,
		},
	}
}

func TestAGateRefusalWaitsOnTheChecksItSawFail(t *testing.T) {
	t.Parallel()

	// Arrange
	failedCheck := forge.CI{State: forge.CIFailed, Checks: []forge.Check{{Name: lintCheck, State: forge.CIFailed}}}

	// Act
	got, _ := delivering(refusedFor(failedCheck), 2*time.Minute)(due(t))

	// Assert
	// C82: the refusal's own CI, so a surface can name the checks.
	if checks := got.CI().Checks; len(checks) != 1 || checks[0].Name != lintCheck {
		t.Errorf("CI = %+v, want the gate's read, naming lint", got.CI())
	}
}

func TestAPullClosedAfterAGateRefusalIsNoLongerTheOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// C39: refused by the gate as it was to be posted, then closed, which a
	// find skips: the pull request listed once is not waited for again.
	refused := deliveredAs(due(t), refusedFor(forge.CI{State: forge.CIFailed}), 2*time.Minute)

	// Act
	got, _ := hearing(forge.PullRequest{}, false, forge.CINone, 3*time.Minute)(refused)

	// Assert
	if got.Stage() != armed.StageDropped || !errors.Is(got.Reason(), armed.ErrPullChanged) {
		t.Errorf("stepped to %s (%v), want dropped: the pull request is no longer the one",
			stageName(got.Stage()), got.Reason())
	}
}

func TestAnEventOfAnotherRoundChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from  func(*testing.T) armed.Hold
		event step
	}{
		"a late read": {from: running, event: func(hold armed.Hold) (armed.Hold, armed.Next) {
			return hold.Heard(armed.Reading{
				Round: hold.Round() - 1, Pull: heldPull(), Found: true, CI: forge.CI{State: forge.CIPassed},
			})
		}},
		"a late failure": {from: running, event: func(hold armed.Hold) (armed.Hold, armed.Next) {
			return hold.Unheard(armed.Failure{Round: hold.Round() + 1, Err: errSeam, At: after(armed.UnheardPatience)})
		}},
		"a late outcome": {from: due, event: func(hold armed.Hold) (armed.Hold, armed.Next) {
			return hold.Delivered(armed.Outcome{Round: hold.Round() - 1, Err: nil})
		}},
		"an outcome of no post":  {from: running, event: delivering(nil, 2*time.Minute)},
		"a read once posted":     {from: posted, event: hearing(heldPull(), true, forge.CIFailed, 3*time.Minute)},
		"a read while posting":   {from: due, event: hearing(heldPull(), true, forge.CIFailed, 3*time.Minute)},
		"a failure once stopped": {from: stopped, event: failing(errSeam, 3*time.Minute)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := tt.from(t)

			// Act
			got, next := tt.event(hold)

			// Assert
			if !reflect.DeepEqual(got, hold) || next != armed.NextNothing {
				t.Errorf("stepped to %s, asking for %s; want it unchanged, asking for nothing",
					stageName(got.Stage()), nextName(next))
			}
		})
	}
}
