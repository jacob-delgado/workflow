// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

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
	CI      forge.CI
	Verdict Verdict
	Err     error
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

		green, verdict, err := seams.round(ctx, number, &state)
		if err != nil {
			return Green{}, err
		}

		if verdict == VerdictDue {
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
// VerdictWaiting, to be read again, its time not counted as quiet; a pull
// request no longer armed, and no check reported within patience, end the
// wait.
func (s AwaitSeams) round(ctx context.Context, number int, state *awaitState) (Green, Verdict, error) {
	green, err := s.readPull(ctx, number, &state.listed)
	if errors.Is(err, ErrPullChanged) || errors.Is(err, ErrMergedFirst) {
		return Green{}, VerdictWaiting, err
	}

	if err == nil {
		var ci forge.CI

		ci, err = s.CheckCI(ctx, green.Pull, green.Head)
		if err == nil {
			return s.settle(green, ci, &state.watch)
		}
	}

	if ctx.Err() != nil {
		return Green{}, VerdictWaiting, fmt.Errorf("waiting for CI to pass: %w", ctx.Err())
	}

	state.watch = state.watch.Unheard()
	s.hear(Heard{CI: forge.CI{}, Verdict: VerdictWaiting, Err: err})

	return Green{}, VerdictWaiting, nil
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
func (s AwaitSeams) settle(green Green, ci forge.CI, watch *CIWatch) (Green, Verdict, error) {
	var verdict Verdict

	*watch, verdict = watch.Read(ci, s.Now())
	s.hear(Heard{CI: ci, Verdict: verdict, Err: nil})

	if verdict == VerdictNoCI {
		return Green{}, verdict, ErrNoCI
	}

	return green, verdict, nil
}

// hear tells Heard of a read, when it listens.
func (s AwaitSeams) hear(heard Heard) {
	if s.Heard != nil {
		s.Heard(heard)
	}
}
