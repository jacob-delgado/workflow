// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"errors"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Arm arms the announcement arming describes at armedAt, and asks for a read at
// once. An idle or ended hold is armed afresh, in a new round: listing a
// pull request just opened, else awaiting its checks. A waiting hold of the
// same pull request and moment keeps its stage and round and takes the new
// text, channel and tags. One of another, asked for, replaces it, stopping
// it; armed on its own, it never does, and is OtherHeldError. A hold being
// posted is ErrBeingPosted.
func (h Hold) Arm(armedAt time.Time, arming Arming) (Armed, error) {
	switch {
	case h.stage == StageDue:
		return Armed{Hold: h, Next: NextNothing, Replaced: nil}, ErrBeingPosted
	case h.stage.Ended():
		return Armed{Hold: h.armedAfresh(armedAt, arming), Next: NextRead, Replaced: nil}, nil
	case h.Holds(arming.Delivery.Made()):
		return Armed{Hold: h.retold(arming), Next: NextNothing, Replaced: nil}, nil
	case arming.Automatic:
		return Armed{Hold: h, Next: NextNothing, Replaced: nil}, OtherHeldError{Held: h.Pull()}
	default:
		replaced := h.ended(StageStopped, ErrStopped)

		return Armed{Hold: h.armedAfresh(armedAt, arming), Next: NextRead, Replaced: &replaced}, nil
	}
}

// Start is an idle hold armed with arming at armedAt, asking for a read at
// once: what Arm answers for a hold never armed, which it never refuses.
func Start(armedAt time.Time, arming Arming) Armed {
	return Armed{Hold: Hold{}.armedAfresh(armedAt, arming), Next: NextRead, Replaced: nil}
}

// armedAfresh is the hold armed with arming at armedAt, in the next round.
func (h Hold) armedAfresh(armedAt time.Time, arming Arming) Hold {
	stage := StageAwaitingChecks
	if arming.Opened {
		stage = StageListing
	}

	return Hold{
		stage: stage, round: h.round + 1, asked: 0, arming: arming, watch: WatchFrom(armedAt),
		unheardSince: time.Time{}, ci: forge.CI{}, reason: nil,
	}
}

// retold is the hold posting what arming says, to where it says, tagging
// whom it says, as it stands otherwise.
func (h Hold) retold(arming Arming) Hold {
	h.arming.Delivery.Channel = arming.Delivery.Channel
	h.arming.Delivery.Text = arming.Delivery.Text
	h.arming.Delivery.Mentions = arming.Delivery.Mentions
	h.arming.Unasked, h.arming.Automatic = arming.Unasked, arming.Automatic

	return h
}

// Asked is the hold with a read of it asked for, and that read, which
// Seams.Read makes. Only the read asked last in a round is heard: one asked
// before it read the pull request and its CI as they stood earlier, so,
// answering late, it leaves the hold as it was.
func (h Hold) Asked() (Hold, Ask) {
	h.asked++

	return h, Ask{round: h.round, read: h.asked, pull: h.Pull(), branch: h.Branch()}
}

// Hear steps the hold on a read: Unheard when failure carries an error, else
// Heard on reading.
func (h Hold) Hear(reading Reading, failure Failure) (Hold, Next) {
	if failure.Err != nil {
		return h.Unheard(failure)
	}

	return h.Heard(reading)
}

// Heard steps a waiting hold on a read of its pull request. Listing, one not
// found is waited for within UnheardPatience. Once found, a pull request
// that is another, none, closed, merged or merging drops it; otherwise its
// CI says how it waits, or that it is due, its delivery's pull request and
// head taken from the read, or drops it: no check reported within
// NoCIPatience, or CI that ran unchanged for RunningPatience. A read asked
// before the last in its round is not heard.
func (h Hold) Heard(reading Reading) (Hold, Next) {
	if !h.hears(reading.Ask) {
		return h, NextNothing
	}

	if h.stage == StageListing && !reading.Found {
		return h.unheard(ErrNotListedYet, reading.At)
	}

	err := StillArmed(h.Pull(), reading.Pull, reading.Found)
	if err != nil {
		return h.ended(StageDropped, err), NextNothing
	}

	return h.readCI(reading)
}

// readCI steps the hold on the CI reading heard. A read that leaves it waiting
// starts its UnheardPatience again; one that makes it due does not, so a post
// tried again past failures, each after a read that heard CI pass, is still
// given up on once the patience has passed since the first failure.
func (h Hold) readCI(reading Reading) (Hold, Next) {
	watch, verdict := h.watch.Read(reading.CI, reading.At)
	h.watch, h.ci, h.reason = watch, reading.CI, nil

	givenUp := givenUpOn(verdict)
	if givenUp != nil {
		return h.ended(StageDropped, givenUp), NextNothing
	}

	h.stage = stageOn(verdict)
	if verdict != VerdictDue {
		h.unheardSince = time.Time{}

		return h, NextReadLater
	}

	h.arming.Delivery.Pull, h.arming.Delivery.Head = reading.Pull, reading.Pull.HeadCommit

	return h, NextDeliver
}

// stageOn is the stage a hold stands at on a verdict: no check within
// patience, or CI unchanged past its own, drops it. A map, not a switch, so
// there is no last arm gobco can never see false.
func stageOn(verdict Verdict) Stage {
	return map[Verdict]Stage{
		VerdictWaiting:        StageRunning,
		VerdictAwaitingFix:    StageAwaitingFix,
		VerdictAwaitingChecks: StageAwaitingChecks,
		VerdictDue:            StageDue,
		VerdictNoCI:           StageDropped,
		VerdictRunningTooLong: StageDropped,
	}[verdict]
}

// givenUpOn is why a hold is given up on a verdict, or nil for one it waits
// or posts on. A map, not a switch, for the reason stageOn's is.
func givenUpOn(verdict Verdict) error {
	return map[Verdict]error{
		VerdictWaiting:        nil,
		VerdictAwaitingChecks: nil,
		VerdictAwaitingFix:    nil,
		VerdictDue:            nil,
		VerdictNoCI:           ErrNoCI,
		VerdictRunningTooLong: ErrRunningTooLong,
	}[verdict]
}

// Unheard steps a waiting hold on a read that failed: it waits to read
// again, the time not counted as quiet, until UnheardPatience has passed
// since the first failure since one was heard, and then drops it with
// ErrUnheard. Listing, it stays listing. A read asked before the last in its
// round is not heard failing either.
func (h Hold) Unheard(failure Failure) (Hold, Next) {
	if !h.hears(failure.Ask) {
		return h, NextNothing
	}

	return h.unheard(failure.Err, failure.At)
}

// unheard is the hold after err at at, a failure to hear: waiting to try
// again, or out of patience.
func (h Hold) unheard(err error, at time.Time) (Hold, Next) {
	if h.unheardSince.IsZero() {
		h.unheardSince = at
	}

	if at.Sub(h.unheardSince) >= UnheardPatience {
		return h.ended(StageDropped, UnheardError{Last: err}), NextNothing
	}

	h.watch, h.reason = h.watch.Unheard(), err
	if h.stage != StageListing {
		h.stage = StageUnheard
	}

	return h, NextReadLater
}

// Delivered steps a hold being posted on its post's outcome: posted, the
// store's failure to confirm it kept as its reason; a check the gate saw
// fail, which waits on a fix; a gate that could not read CI, a store that
// could not claim it or a service that asked it to wait, each waited past
// within UnheardPatience; claimed elsewhere first; or, any other way it
// went, dropped, the claim kept or given up as loop.Deliver decided.
func (h Hold) Delivered(outcome Outcome) (Hold, Next) {
	if outcome.Round != h.round || h.stage != StageDue {
		return h, NextNothing
	}

	return afterPost(outcome.Kind)(h, outcome)
}

// afterPost is how a hold steps on a post that went as kind. It tries again
// only past what a wait can change: CI the gate could not read, a claim the
// store could not take, or a service that asked it to wait. One that may
// have gone out never is: its claim is kept, so a second try would only find
// it held. A map, not a switch, so exhaustive names a kind added later until
// it is placed here; one with no entry, of no kind loop.Deliver names, drops
// the hold.
func afterPost(kind loop.Delivered) func(Hold, Outcome) (Hold, Next) {
	step, placed := map[loop.Delivered]func(Hold, Outcome) (Hold, Next){
		loop.DeliveredFailed:            Hold.droppedOn,
		loop.DeliveredPosted:            Hold.postedOn,
		loop.DeliveredUnconfirmed:       Hold.postedOn,
		loop.DeliveredCIFailed:          Hold.awaitingFixOn,
		loop.DeliveredCIUnread:          Hold.triedAgainOn,
		loop.DeliveredNotClaimed:        Hold.triedAgainOn,
		loop.DeliveredElsewhere:         Hold.elsewhereOn,
		loop.DeliveredRefused:           Hold.droppedOn,
		loop.DeliveredDeferred:          Hold.triedAgainOn,
		loop.DeliveredDeferredUnclaimed: Hold.triedAgainOn,
		loop.DeliveredUncertain:         Hold.droppedOn,
	}[kind]
	if !placed {
		return Hold.droppedOn
	}

	return step
}

// postedOn is the hold posted by outcome, the store's failure to confirm it,
// if any, kept as its reason.
func (h Hold) postedOn(outcome Outcome) (Hold, Next) {
	return h.ended(StagePosted, outcome.Err), NextNothing
}

// awaitingFixOn is the hold waiting on a fix the gate's refusal in outcome
// says is needed.
func (h Hold) awaitingFixOn(outcome Outcome) (Hold, Next) {
	refusal, _ := errors.AsType[loop.GateRefusalError](outcome.Err)

	return h.refusedByGate(refusal, outcome.At), NextReadLater
}

// triedAgainOn is the hold waiting past outcome's failure, which a wait can
// change, within UnheardPatience.
func (h Hold) triedAgainOn(outcome Outcome) (Hold, Next) {
	return h.reclaimedFrom(outcome.Kind).unheard(outcome.Err, outcome.At)
}

// elsewhereOn is the hold whose announcement outcome found claimed
// elsewhere first.
func (h Hold) elsewhereOn(outcome Outcome) (Hold, Next) {
	return h.ended(StageElsewhere, outcome.Err), NextNothing
}

// droppedOn is the hold dropped on outcome, the claim kept or given up as
// loop.Deliver decided.
func (h Hold) droppedOn(outcome Outcome) (Hold, Next) {
	return h.ended(StageDropped, outcome.Err), NextNothing
}

// reclaimedFrom is a hold asked for again whose post, which went as kind,
// re-claimed it: its next post re-claims from the generation that one took.
// A post the service asked to wait is the one way it tries again having
// taken the claim, which loop.Redeliver keeps; any other hold is unchanged.
func (h Hold) reclaimedFrom(kind loop.Delivered) Hold {
	if h.arming.Seen > 0 && kind == loop.DeliveredDeferred {
		h.arming.Seen++
	}

	return h
}

// refusedByGate is the hold waiting on a fix the gate's refusal, at at, says
// is needed: its read counts as one that reported a check.
func (h Hold) refusedByGate(refusal loop.GateRefusalError, at time.Time) Hold {
	h.watch, _ = h.watch.Read(refusal.CI, at)
	h.stage, h.ci, h.unheardSince, h.reason = StageAwaitingFix, refusal.CI, time.Time{}, refusal

	return h
}

// Stop stops a waiting hold. One being posted is ErrBeingPosted, and one not
// waiting ErrNothingWaiting; neither is changed.
func (h Hold) Stop() (Hold, error) {
	switch {
	case h.stage == StageDue:
		return h, ErrBeingPosted
	case h.stage.Ended():
		return h, ErrNothingWaiting
	default:
		return h.ended(StageStopped, ErrStopped), nil
	}
}

// Superseded is a waiting hold of made posted in its place, by an
// announcement asked for now that went out; any other hold is unchanged.
func (h Hold) Superseded(made loop.Announced) Hold {
	if !h.stage.Waiting() || !h.Holds(made) {
		return h
	}

	return h.ended(StagePosted, nil)
}

// Retire stops a waiting hold whose session or server ended, with ErrLost.
// Any other is unchanged: a post under way finishes, and its claim guards
// it.
func (h Hold) Retire() Hold {
	if !h.stage.Waiting() {
		return h
	}

	return h.ended(StageStopped, ErrLost)
}

// hears reports whether the read asked as ask applies: the read asked last
// in the hold's own round, while it waits.
func (h Hold) hears(ask Ask) bool {
	return ask.round == h.round && ask.read == h.asked && h.stage.Waiting()
}

// ended is the hold at stage, done with, for reason.
func (h Hold) ended(stage Stage, reason error) Hold {
	h.stage, h.reason = stage, reason

	return h
}
