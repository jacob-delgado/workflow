// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestTheStagesThatWaitAndThatEnded(t *testing.T) {
	t.Parallel()

	cases := map[armed.Stage]struct{ waiting, ended bool }{
		armed.StageIdle: {ended: true}, armed.StageListing: {waiting: true},
		armed.StageAwaitingChecks: {waiting: true}, armed.StageRunning: {waiting: true},
		armed.StageAwaitingFix: {waiting: true}, armed.StageUnheard: {waiting: true}, armed.StageDue: {},
		armed.StagePosted: {ended: true}, armed.StageElsewhere: {ended: true}, armed.StageDropped: {ended: true},
		armed.StageStopped: {ended: true},
	}

	for stage, want := range cases {
		t.Run(stageName(stage), func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if stage.Waiting() != want.waiting || stage.Ended() != want.ended {
				t.Errorf("waiting %t, ended %t; want %t, %t", stage.Waiting(), stage.Ended(), want.waiting, want.ended)
			}
		})
	}
}

func TestArmingAnIdleOrEndedHoldStartsARound(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from  func(*testing.T) armed.Hold
		arm   armed.Arming
		stage armed.Stage
		round int
	}{
		"never armed, asked":              {from: idle, arm: arming(), stage: armed.StageAwaitingChecks, round: 1},
		"never armed, just opened (P025)": {from: idle, arm: opening(), stage: armed.StageListing, round: 1},
		"once posted":                     {from: posted, arm: arming(), stage: armed.StageAwaitingChecks, round: 2},
		"once dropped":                    {from: dropped, arm: opening(), stage: armed.StageListing, round: 2},
		"once stopped":                    {from: stopped, arm: arming(), stage: armed.StageAwaitingChecks, round: 2},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := tt.from(t)

			// Act
			got, err := hold.Arm(armedAt(), tt.arm)

			// Assert
			if err != nil || got.Hold.Stage() != tt.stage || got.Hold.Round() != tt.round || got.Next != armed.NextRead {
				t.Errorf("Arm = %s in round %d asking for %s (%v); want %s in round %d, asking for a read",
					stageName(got.Hold.Stage()), got.Hold.Round(), nextName(got.Next), err, stageName(tt.stage), tt.round)
			}

			if got.Replaced != nil || got.Hold.Reason() != nil || got.Hold.Branch() != branchName {
				t.Errorf("Arm replaced %v, with reason %v on %q; want nothing replaced, no reason, on %q",
					got.Replaced, got.Hold.Reason(), got.Hold.Branch(), branchName)
			}
		})
	}
}

func TestStartingArmsAsAnIdleHoldIsArmed(t *testing.T) {
	t.Parallel()

	// Arrange
	want, err := armed.Hold{}.Arm(armedAt(), opening())
	if err != nil {
		t.Fatalf("arming an idle hold: %v", err)
	}

	// Act
	got := armed.Start(armedAt(), opening())

	// Assert
	if got.Hold.Stage() != want.Hold.Stage() || got.Hold.Round() != want.Hold.Round() || got.Next != want.Next ||
		got.Replaced != nil || got.Hold.Branch() != branchName {
		t.Errorf("Start = %s in round %d asking for %s; want %s in round %d asking for %s, as Arm does",
			stageName(got.Hold.Stage()), got.Hold.Round(), nextName(got.Next),
			stageName(want.Hold.Stage()), want.Hold.Round(), nextName(want.Next))
	}
}

func TestArmingTheHeldAnnouncementAgainRetellsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	hold := running(t)
	again := arming()
	again.Delivery.Text, again.Delivery.Channel = "Redact the token, take two", "#release"

	// Act
	got, err := hold.Arm(after(3*time.Minute), again)

	// Assert
	if err != nil || got.Next != armed.NextNothing || got.Hold.Stage() != armed.StageRunning ||
		got.Hold.Round() != hold.Round() {
		t.Errorf("Arm = %s in round %d asking for %s (%v); want it running as it was, asking for nothing",
			stageName(got.Hold.Stage()), got.Hold.Round(), nextName(got.Next), err)
	}

	if delivery := got.Hold.Delivery(); delivery.Text != again.Delivery.Text || delivery.Channel != "#release" {
		t.Errorf("delivery = %q to %q, want the new text to #release", delivery.Text, delivery.Channel)
	}
}

func TestArmingAnotherPullRequestAsAskedReplacesTheHeldOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// P113: an asked arm replaces another pull request's hold, and says so.
	hold := running(t)
	another := arming()
	another.Delivery = deliveryOf(otherPull())

	// Act
	got, err := hold.Arm(after(3*time.Minute), another)

	// Assert
	if err != nil || got.Hold.Pull() != 12 || got.Hold.Stage() != armed.StageAwaitingChecks ||
		got.Hold.Round() != hold.Round()+1 || got.Next != armed.NextRead {
		t.Errorf("Arm = #%d %s in round %d asking for %s (%v); want #12 awaiting checks in the next round, read now",
			got.Hold.Pull(), stageName(got.Hold.Stage()), got.Hold.Round(), nextName(got.Next), err)
	}

	if got.Replaced == nil || got.Replaced.Pull() != 9 || got.Replaced.Stage() != armed.StageStopped ||
		!errors.Is(got.Replaced.Reason(), armed.ErrStopped) {
		t.Errorf("Arm replaced %+v, want #9 stopped", got.Replaced)
	}
}

func TestArmingAnotherMomentAsAskedReplacesTheHeldOne(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := arming()
	merged.Delivery.Moment = messaging.MomentMerged

	// Act
	got, err := running(t).Arm(after(3*time.Minute), merged)

	// Assert
	if err != nil || got.Replaced == nil || got.Hold.Delivery().Moment != messaging.MomentMerged {
		t.Errorf("Arm = %+v (%v), want the merge armed, the ready one replaced", got, err)
	}
}

func TestArmingOnItsOwnNeverReplacesAnotherHold(t *testing.T) {
	t.Parallel()

	// Arrange
	// P113: the arm on open leaves another pull request's hold waiting, and
	// says which.
	hold := running(t)
	another := opening()
	another.Delivery = deliveryOf(otherPull())

	// Act
	got, err := hold.Arm(after(3*time.Minute), another)

	// Assert
	var other armed.OtherHeldError
	if !errors.As(err, &other) || other.Held != 9 || !errors.Is(err, armed.ErrArmedOther) {
		t.Errorf("Arm = %v, want ErrArmedOther naming #9", err)
	}

	if !reflect.DeepEqual(got.Hold, hold) || got.Next != armed.NextNothing || got.Replaced != nil {
		t.Errorf("Arm changed the hold to #%d %s, asking for %s; want it left as it was",
			got.Hold.Pull(), stageName(got.Hold.Stage()), nextName(got.Next))
	}
}

func TestArmingWhileItIsBeingPostedIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	hold := due(t)

	// Act
	got, err := hold.Arm(after(3*time.Minute), arming())

	// Assert
	if !errors.Is(err, armed.ErrBeingPosted) || !reflect.DeepEqual(got.Hold, hold) || got.Next != armed.NextNothing {
		t.Errorf("Arm = %s asking for %s (%v), want it refused as being posted, unchanged",
			stageName(got.Hold.Stage()), nextName(got.Next), err)
	}
}

func TestStoppingAWaitingHoldStopsIt(t *testing.T) {
	t.Parallel()

	for name, from := range map[string]func(*testing.T) armed.Hold{
		stageName(armed.StageListing): listing, stageName(armed.StageRunning): running,
		stageName(armed.StageAwaitingFix): awaitingFix, stageName(armed.StageUnheard): unheard,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := from(t).Stop()

			// Assert
			if err != nil || got.Stage() != armed.StageStopped || !errors.Is(got.Reason(), armed.ErrStopped) {
				t.Errorf("Stop = %s (%v), %v; want stopped", stageName(got.Stage()), got.Reason(), err)
			}
		})
	}
}

func TestStoppingAHoldThatDoesNotWaitChangesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) armed.Hold
		want error
	}{
		stageName(armed.StageDue):    {from: due, want: armed.ErrBeingPosted},
		stageName(armed.StageIdle):   {from: idle, want: armed.ErrNothingWaiting},
		stageName(armed.StagePosted): {from: posted, want: armed.ErrNothingWaiting},
		// C70: a dropped hold keeps its reason.
		stageName(armed.StageDropped): {from: dropped, want: armed.ErrNothingWaiting},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := tt.from(t)

			// Act
			got, err := hold.Stop()

			// Assert
			if !errors.Is(err, tt.want) || !reflect.DeepEqual(got, hold) {
				t.Errorf("Stop = %s (%v), %v; want it unchanged, refused with %v",
					stageName(got.Stage()), got.Reason(), err, tt.want)
			}
		})
	}
}

func TestRetiringAWaitingHoldLosesIt(t *testing.T) {
	t.Parallel()

	// Act
	got := awaitingChecks(t).Retire()

	// Assert
	if got.Stage() != armed.StageStopped || !errors.Is(got.Reason(), armed.ErrLost) {
		t.Errorf("Retire = %s (%v), want stopped, lost", stageName(got.Stage()), got.Reason())
	}
}

func TestRetiringAHoldThatDoesNotWaitChangesNothing(t *testing.T) {
	t.Parallel()

	for name, from := range map[string]func(*testing.T) armed.Hold{
		stageName(armed.StageDue): due, stageName(armed.StageIdle): idle, stageName(armed.StagePosted): posted,
		stageName(armed.StageDropped): dropped,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := from(t)

			// Act
			got := hold.Retire()

			// Assert
			if !reflect.DeepEqual(got, hold) {
				t.Errorf("Retire = %s (%v), want it unchanged", stageName(got.Stage()), got.Reason())
			}
		})
	}
}

func TestAnAnnouncementPostedNowSupersedesTheHoldOfIt(t *testing.T) {
	t.Parallel()

	// Act
	got := running(t).Superseded(loop.Announced{Pull: 9, Moment: messaging.MomentReady})

	// Assert
	if got.Stage() != armed.StagePosted || got.Reason() != nil {
		t.Errorf("Superseded = %s (%v), want posted", stageName(got.Stage()), got.Reason())
	}
}

func TestAnAnnouncementPostedNowLeavesAnyOtherHold(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		from func(*testing.T) armed.Hold
		made loop.Announced
	}{
		"#12 posted":       {from: running, made: loop.Announced{Pull: 12, Moment: messaging.MomentReady}},
		"#9 merged posted": {from: running, made: loop.Announced{Pull: 9, Moment: messaging.MomentMerged}},
		"#9 due":           {from: due, made: loop.Announced{Pull: 9, Moment: messaging.MomentReady}},
		"#9 given up on":   {from: dropped, made: loop.Announced{Pull: 9, Moment: messaging.MomentReady}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			hold := tt.from(t)

			// Act
			got := hold.Superseded(tt.made)

			// Assert
			if !reflect.DeepEqual(got, hold) {
				t.Errorf("Superseded = %s, want it unchanged", stageName(got.Stage()))
			}
		})
	}
}

func TestAHoldIsReadAndAutomaticAsArmed(t *testing.T) {
	t.Parallel()

	// Act
	hold := listing(t)

	// Assert
	if !hold.Automatic() || hold.Branch() != branchName || hold.Pull() != 9 ||
		!hold.Holds(loop.Announced{Pull: 9, Moment: messaging.MomentReady}) {
		t.Errorf("hold = automatic %t on %q for #%d, want automatic on %q for #9",
			hold.Automatic(), hold.Branch(), hold.Pull(), branchName)
	}
}

func TestAHoldKeepsTheCIItLastHeard(t *testing.T) {
	t.Parallel()

	// Arrange
	hold := running(t)
	ci := forge.CI{State: forge.CIRunning, Total: 3, Done: 1}

	// Act
	got, _ := hold.Heard(armed.Reading{Round: hold.Round(), Pull: heldPull(), Found: true, CI: ci, At: after(time.Hour)})

	// Assert
	if got.CI().Total != 3 || got.CI().Done != 1 {
		t.Errorf("CI = %+v, want the read's, 1 of 3 done", got.CI())
	}
}
