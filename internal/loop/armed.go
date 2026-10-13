// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// NoCIPatience is how long an announcement armed to go once CI passes waits
// for a pull request that reports no check, counted from the last read that
// reported one, or from arming, before it gives up.
//
// Trade-off TRADE-41: a CI that registers later than this loses its automatic
// post, so a repository with no CI is not polled for as long as it waits.
const NoCIPatience = 10 * time.Minute

var (
	// ErrNoCI gives up on an armed announcement whose pull request reported no
	// check within NoCIPatience: it is announced by hand.
	ErrNoCI = errors.New("no checks were reported on the pull request")
	// ErrPullChanged gives up on an armed announcement whose branch's pull
	// request is another one, or none.
	ErrPullChanged = errors.New("the branch's pull request is no longer the one announced")
	// ErrMergedFirst gives up on an armed announcement whose pull request
	// merged before its CI passed.
	ErrMergedFirst = errors.New("the pull request merged before its CI passed")
	// ErrNotListedYet is a find that does not see a pull request just opened,
	// which a forge may list only a moment later: a read to make again.
	ErrNotListedYet = errors.New("the forge does not list the pull request yet")
)

// Armed is how an announcement armed to go once CI passes stands on a read of
// its pull request's CI.
type Armed int

const (
	// ArmedWaiting waits while CI runs.
	ArmedWaiting Armed = iota
	// ArmedAwaitingChecks waits for the pull request to report a check.
	ArmedAwaitingChecks
	// ArmedAwaitingFix waits for a run that passes, a check having failed.
	ArmedAwaitingFix
	// ArmedDue is to post now: CI passed.
	ArmedDue
	// ArmedNoCI gives up: no check was reported within NoCIPatience.
	ArmedNoCI
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
func (w CIWatch) Read(ci forge.CI, readAt time.Time) (CIWatch, Armed) {
	reported := CIWatch{quietSince: readAt}

	switch ci.State {
	case forge.CIRunning:
		return reported, ArmedWaiting
	case forge.CIFailed:
		return reported, ArmedAwaitingFix
	case forge.CIPassed:
		return reported, ArmedDue
	case forge.CINone:
	}

	if readAt.Sub(w.quietSince) >= NoCIPatience {
		return w, ArmedNoCI
	}

	return w, ArmedAwaitingChecks
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

// AwaitSeams are what AwaitGreen reads and waits through. Pause waits wait
// under ctx, answering ctx's error once it ends; Now tells the time patience
// is counted by. Heard, which may be nil, is told of each read. Delay, when
// set, is waited before the first read, as for a wait taken up again. Opened
// is set for a pull request opened a moment ago, which the forge may not list
// yet: until a find has seen it, one that does not is heard as a read that
// failed, ErrNotListedYet, and read again.
type AwaitSeams struct {
	Branch   func(ctx context.Context) (gitrepo.Branch, error)
	FindPull func(ctx context.Context, branch string) (forge.PullRequest, bool, error)
	CheckCI  func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error)
	Pause    func(ctx context.Context, wait time.Duration) error
	Now      func() time.Time
	Interval time.Duration
	Delay    time.Duration
	Opened   bool
	Heard    func(Heard)
}

// Heard is one read of an armed announcement's CI: what it said and how the
// announcement stands on it, or why the read failed.
type Heard struct {
	CI    forge.CI
	Armed Armed
	Err   error
}

// Green is the pull request whose CI passed, and the head it passed at.
type Green struct {
	Pull forge.PullRequest
	Head string
}

// AwaitGreen reads the branch, its pull request and its CI at once, or once
// Delay has passed, and then every Interval, under ctx, until the CI of pull
// request number passes,
// answering the pull request as read then. It gives up with ErrNoCI,
// ErrPullChanged or ErrMergedFirst, or ends with ctx's error. A read that
// fails is told through Heard and waited past. It starts no goroutine: Pause
// is the caller's wait.
func AwaitGreen(ctx context.Context, seams AwaitSeams, number int) (Green, error) {
	state := awaitState{watch: WatchFrom(seams.Now()), listed: !seams.Opened}

	for wait := seams.Delay; ; wait = seams.Interval {
		if wait > 0 {
			err := seams.Pause(ctx, wait)
			if err != nil {
				return Green{}, err
			}
		}

		green, armed, err := seams.round(ctx, number, &state)
		if err != nil {
			return Green{}, err
		}

		if armed == ArmedDue {
			return green, nil
		}
	}
}

// awaitState is how a wait for CI to pass stands between its reads: its
// patience with no check, and whether a find has listed the pull request.
type awaitState struct {
	watch  CIWatch
	listed bool
}

// round reads the branch, its pull request and its CI once, and says how the
// armed announcement stands on them. A read that fails is heard and answers
// ArmedWaiting, to be read again; a pull request no longer armed, and no
// check reported within patience, end the wait.
func (s AwaitSeams) round(ctx context.Context, number int, state *awaitState) (Green, Armed, error) {
	green, err := s.readPull(ctx, number, &state.listed)
	if errors.Is(err, ErrPullChanged) || errors.Is(err, ErrMergedFirst) {
		return Green{}, ArmedWaiting, err
	}

	if err == nil {
		var ci forge.CI

		ci, err = s.CheckCI(ctx, green.Pull, green.Head)
		if err == nil {
			return s.settle(green, ci, &state.watch)
		}
	}

	if ctx.Err() != nil {
		return Green{}, ArmedWaiting, fmt.Errorf("waiting for CI to pass: %w", ctx.Err())
	}

	s.hear(Heard{CI: forge.CI{}, Armed: ArmedWaiting, Err: err})

	return Green{}, ArmedWaiting, nil
}

// readPull reads the branch afresh, for its head, and its pull request,
// answering ErrPullChanged or ErrMergedFirst when it is no longer number, and
// ErrNotListedYet for one not found that no find has listed yet, noting in
// listed a find that has.
func (s AwaitSeams) readPull(ctx context.Context, number int, listed *bool) (Green, error) {
	branch, err := s.Branch(ctx)
	if err != nil {
		return Green{}, fmt.Errorf("reading the branch: %w", err)
	}

	pull, found, err := s.FindPull(ctx, branch.Name)
	if err != nil {
		return Green{}, fmt.Errorf("reading the pull request: %w", err)
	}

	if !found && !*listed {
		return Green{}, ErrNotListedYet
	}

	*listed = true

	err = StillArmed(number, pull, found)
	if err != nil {
		return Green{}, err
	}

	return Green{Pull: pull, Head: branch.Head}, nil
}

// settle is how the armed announcement stands on ci, heard: ErrNoCI once no
// check was reported within patience.
func (s AwaitSeams) settle(green Green, ci forge.CI, watch *CIWatch) (Green, Armed, error) {
	var armed Armed

	*watch, armed = watch.Read(ci, s.Now())
	s.hear(Heard{CI: ci, Armed: armed, Err: nil})

	if armed == ArmedNoCI {
		return Green{}, armed, ErrNoCI
	}

	return green, armed, nil
}

// hear tells Heard of a read, when it listens.
func (s AwaitSeams) hear(heard Heard) {
	if s.Heard != nil {
		s.Heard(heard)
	}
}
