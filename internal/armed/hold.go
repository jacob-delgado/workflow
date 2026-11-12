// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Stage is where an announcement armed to go once CI passes stands.
type Stage int

const (
	// StageIdle has never been armed. It is the zero stage.
	StageIdle Stage = iota
	// StageListing waits for the forge to list a pull request just opened.
	StageListing
	// StageAwaitingChecks waits for the pull request to report a check.
	StageAwaitingChecks
	// StageRunning waits while CI runs, for RunningPatience at most while
	// it runs unchanged.
	StageRunning
	// StageAwaitingFix waits for a run that passes, a check having failed.
	StageAwaitingFix
	// StageUnheard waits past a read, or a post, that failed, to try again.
	StageUnheard
	// StageDue is being posted: CI passed.
	StageDue
	// StagePosted was posted.
	StagePosted
	// StageElsewhere was not posted: another session or process claimed it
	// first.
	StageElsewhere
	// StageDropped was given up on, for the reason the hold keeps.
	StageDropped
	// StageStopped was stopped before it was posted: asked to, replaced, or
	// its session ended.
	StageStopped
)

// Waiting reports whether an announcement at this stage waits for its CI:
// listing, awaiting checks, running, awaiting a fix, or unheard.
func (s Stage) Waiting() bool {
	return s >= StageListing && s <= StageUnheard
}

// Ended reports whether an announcement at this stage is done with, or was
// never armed: neither waiting nor being posted.
func (s Stage) Ended() bool {
	return !s.Waiting() && s != StageDue
}

// Next is the effect a step asks of whoever drives the hold.
type Next int

const (
	// NextNothing asks for nothing: the hold waits for an event, or has
	// ended.
	NextNothing Next = iota
	// NextRead asks for a read of the pull request and its CI now.
	NextRead
	// NextReadLater asks for a read an interval from now.
	NextReadLater
	// NextDeliver asks for the held delivery to be posted, through
	// loop.Deliver.
	NextDeliver
)

// Arming is what an announcement is armed with: the delivery it posts — its
// head is filled from the pull request read as CI passes — the branch every
// read finds its pull request by, whether that pull request was opened a
// moment ago, which a forge may not list yet, whether its tags are proposed
// as it posts, asked of no one, and whether it was armed on its own rather
// than asked for. Seen, above zero, is the generation of the claim an
// announcement asked for again re-claims, as loop.Redeliver does; zero claims
// it afresh.
type Arming struct {
	Delivery  loop.Delivery
	Branch    string
	Opened    bool
	Unasked   bool
	Automatic bool
	Seen      int
}

// Hold is one announcement armed to go once its pull request's CI passes: a
// value each event steps to the next, which every surface keeps and drives
// by its own clock. The zero Hold is idle. Every event carries the round it
// was asked in, and one for another round, or that arrives at a stage it
// does not apply to, leaves the hold as it was and asks for nothing, so a
// read or a post that answers late is harmless.
type Hold struct {
	stage        Stage
	round        int
	arming       Arming
	watch        CIWatch
	unheardSince time.Time
	ci           forge.CI
	reason       error
}

// Armed is a hold just armed, what it asks for first, and the hold it
// replaced, stopped, when it was asked for over another pull request's.
type Armed struct {
	Hold     Hold
	Next     Next
	Replaced *Hold
}

// Reading is a read of the held pull request in round: the pull request the
// branch's find answered, whether it found one, and its CI, read at the
// pull request's head where it is open and the one held, at At.
type Reading struct {
	Round int
	Pull  forge.PullRequest
	Found bool
	CI    forge.CI
	At    time.Time
}

// Failure is a read in round that failed, at At, and why. A Failure with no
// error is no failure: the read was heard.
type Failure struct {
	Round int
	Err   error
	At    time.Time
}

// Outcome is how a post of the held delivery in round went, at At:
// loop.Deliver's answer.
type Outcome struct {
	Round int
	Err   error
	At    time.Time
}

// Stage is where the hold stands.
func (h Hold) Stage() Stage {
	return h.stage
}

// Round is the round the hold is in: one more each time it is armed afresh.
func (h Hold) Round() int {
	return h.round
}

// Delivery is what the hold posts once CI passes, its pull request and head
// as read then once it is due.
func (h Hold) Delivery() loop.Delivery {
	return h.arming.Delivery
}

// Pull is the number of the pull request held.
func (h Hold) Pull() int {
	return h.arming.Delivery.Pull.Number
}

// Branch is the branch every read finds the held pull request by.
func (h Hold) Branch() string {
	return h.arming.Branch
}

// Automatic reports whether the hold was armed on its own, as its pull
// request opened, rather than asked for.
func (h Hold) Automatic() bool {
	return h.arming.Automatic
}

// CI is the CI the hold last heard: from a read, or from the gate that
// refused its post.
func (h Hold) CI() forge.CI {
	return h.ci
}

// Reason is why the hold stands where it does, or nil: why it was dropped or
// stopped, why it was not posted elsewhere, the read or post that failed
// while it waits again, the forge not listing it yet, the gate's refusal it
// waits on a fix past, or, posted, the store's failure to confirm it.
func (h Hold) Reason() error {
	return h.reason
}

// Holds reports whether the hold is of made: its pull request at its moment.
func (h Hold) Holds(made loop.Announced) bool {
	return h.arming.Delivery.Made() == made
}
