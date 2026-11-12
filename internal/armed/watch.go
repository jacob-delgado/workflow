// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package armed is an announcement armed to go once its pull request's CI
// passes: the watch over that CI, and whether the pull request armed is still
// the one to announce, which the terminal, the web and the command line's
// waits share.
package armed

import (
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// NoCIPatience is how long an announcement armed to go once CI passes waits
// for a pull request that reports no check, counted across the reads that were
// heard since the last that reported one, or since arming, before it gives up.
//
// Trade-off TRADE-41: a CI that registers later than ten minutes of reads that
// heard no check loses its automatic post, so a repository with no CI is not
// polled for as long as it waits.
const NoCIPatience = 10 * time.Minute

// UnheardPatience is how long an announcement armed to go once CI passes
// waits past reads, and posts, that failed — a forge, store or messaging
// service unreachable, busy or asking it to wait — and for the forge to list a
// pull request just opened, counted from the first such failure since a read
// was heard, before it gives up with ErrUnheard.
//
// Trade-off TRADE-59: an outage longer than ten minutes loses the automatic
// post, so a wait whose reads always fail ends, on every surface, rather than
// polling for as long as its session runs.
const UnheardPatience = 10 * time.Minute

// Verdict is how an announcement armed to go once CI passes stands on a read
// of its pull request's CI.
type Verdict int

const (
	// VerdictWaiting waits while CI runs.
	VerdictWaiting Verdict = iota
	// VerdictAwaitingChecks waits for the pull request to report a check.
	VerdictAwaitingChecks
	// VerdictAwaitingFix waits for a run that passes, a check having failed.
	VerdictAwaitingFix
	// VerdictDue is to post now: CI passed.
	VerdictDue
	// VerdictNoCI gives up: no check was reported within NoCIPatience.
	VerdictNoCI
)

// CIWatch is the watch over an armed announcement's CI: how long the reads
// heard since the last that reported a check have reported none, and when the
// last read was heard, which the next one counts from. A read that failed
// breaks the span, so an outage is never counted as quiet.
type CIWatch struct {
	quiet     time.Duration
	lastHeard time.Time
}

// WatchFrom is the watch of an announcement armed at armedAt, which counts as
// heard: the first read counts from it.
func WatchFrom(armedAt time.Time) CIWatch {
	return CIWatch{quiet: 0, lastHeard: armedAt}
}

// Read is how the armed announcement stands on ci, heard at readAt, and the
// watch to read the next one with: a check reported, running, failed or
// passed, starts its patience again, and none adds the time since the last
// read heard, if that read was heard too.
func (w CIWatch) Read(ci forge.CI, readAt time.Time) (CIWatch, Verdict) {
	if ci.State != forge.CINone {
		return CIWatch{quiet: 0, lastHeard: readAt}, verdictOn(ci.State)
	}

	quiet := w.quiet
	if !w.lastHeard.IsZero() {
		quiet += readAt.Sub(w.lastHeard)
	}

	heard := CIWatch{quiet: quiet, lastHeard: readAt}
	if quiet >= NoCIPatience {
		return heard, VerdictNoCI
	}

	return heard, VerdictAwaitingChecks
}

// verdictOn is the verdict on a read of CI at state, its patience aside: none
// reported awaits checks. A map, not a switch, so there is no last arm gobco
// can never see false.
func verdictOn(state forge.CIState) Verdict {
	return map[forge.CIState]Verdict{
		forge.CIRunning: VerdictWaiting,
		forge.CIFailed:  VerdictAwaitingFix,
		forge.CIPassed:  VerdictDue,
		forge.CINone:    VerdictAwaitingChecks,
	}[state]
}

// Unheard is the watch after a read that failed: the quiet heard so far is
// kept, and the time since the last read heard is not counted, nor the time
// until the next one.
func (w CIWatch) Unheard() CIWatch {
	return CIWatch{quiet: w.quiet, lastHeard: time.Time{}}
}

// StillArmed reports whether an announcement armed for pull request number
// still stands, given the branch's pull request now: ErrPullChanged when it is
// another one, or none — one closed without merging counts as none — and
// ErrMergedFirst when it merged before its CI passed.
func StillArmed(number int, pull forge.PullRequest, found bool) error {
	switch {
	case !found || pull.Number != number || pull.State == forge.StateClosed:
		return ErrPullChanged
	case pull.State == forge.StateMerged:
		return ErrMergedFirst
	default:
		return nil
	}
}
