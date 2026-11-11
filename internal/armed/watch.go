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
// for a pull request that reports no check, counted from the last read that
// reported one, or from arming, before it gives up.
//
// Trade-off TRADE-41: a CI that registers later than this loses its automatic
// post, so a repository with no CI is not polled for as long as it waits.
const NoCIPatience = 10 * time.Minute

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

// CIWatch is the watch over an armed announcement's CI: when a read last
// reported a check, which its patience with none counts from.
type CIWatch struct {
	quietSince time.Time
}

// WatchFrom is the watch of an announcement armed at armedAt.
func WatchFrom(armedAt time.Time) CIWatch {
	return CIWatch{quietSince: armedAt}
}

// Read is how the armed announcement stands on ci, read at readAt, and the
// watch to read the next one with: a check reported, running, failed or
// passed, starts its patience again.
func (w CIWatch) Read(ci forge.CI, readAt time.Time) (CIWatch, Verdict) {
	reported := CIWatch{quietSince: readAt}

	switch ci.State {
	case forge.CIRunning:
		return reported, VerdictWaiting
	case forge.CIFailed:
		return reported, VerdictAwaitingFix
	case forge.CIPassed:
		return reported, VerdictDue
	case forge.CINone:
	}

	if readAt.Sub(w.quietSince) >= NoCIPatience {
		return w, VerdictNoCI
	}

	return w, VerdictAwaitingChecks
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
