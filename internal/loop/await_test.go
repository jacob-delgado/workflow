// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// awaitInterval is how long each fake pause takes on the fake clock.
const awaitInterval = time.Minute

// awaiting is a forge and a branch an AwaitGreen reads in turn: each round
// takes the next CI answer, the last repeated, on a clock each pause moves.
type awaiting struct {
	cis      []forge.CI
	ciErrs   []error
	pulls    []forge.PullRequest
	rounds   int
	now      time.Time
	heard    []loop.Heard
	heads    []string
	cancel   func()
	pauses   int
	readsAt0 int
	// branchErrs and pullErrs fail the round's read of the branch or of its
	// pull request, without counting a round.
	branchErrs []error
	pullErrs   []error
	tries      int
	// missing, in turn, the last repeated, is whether each find does not see
	// the pull request; finds counts them.
	missing []bool
	finds   int
}

// pick is the round's answer of answers, the last repeated.
func pick[T any](answers []T, round int) T {
	var zero T
	if len(answers) == 0 {
		return zero
	}

	return answers[min(round, len(answers)-1)]
}

// seams are the reads and the pause AwaitGreen is handed.
func (a *awaiting) seams() loop.AwaitSeams {
	return loop.AwaitSeams{
		Branch: func(context.Context) (gitrepo.Branch, error) {
			a.tries++

			return gitrepo.Branch{Name: branchName, Head: "head" + strconv.Itoa(a.rounds)}, pick(a.branchErrs, a.tries-1)
		},
		FindPull: func(context.Context, string) (forge.PullRequest, bool, error) {
			err := pick(a.pullErrs, a.tries-1)

			a.finds++
			if pick(a.missing, a.finds-1) {
				return forge.PullRequest{}, false, err
			}

			if len(a.pulls) == 0 {
				return openPull(), true, err
			}

			return pick(a.pulls, a.rounds), true, err
		},
		CheckCI: func(_ context.Context, _ forge.PullRequest, head string) (forge.CI, error) {
			a.heads = append(a.heads, head)
			if a.pauses == 0 {
				a.readsAt0++
			}

			ci, err := pick(a.cis, a.rounds), pick(a.ciErrs, a.rounds)
			a.rounds++

			return ci, err
		},
		Pause: func(ctx context.Context, wait time.Duration) error {
			a.pauses++
			a.now = a.now.Add(wait)

			if a.cancel != nil && a.pauses == 2 {
				a.cancel()
			}

			return ctx.Err()
		},
		Now:      func() time.Time { return a.now },
		Interval: awaitInterval,
		Heard:    func(heard loop.Heard) { a.heard = append(a.heard, heard) },
	}
}

// states is every CI state in turn.
func states(each ...forge.CIState) []forge.CI {
	cis := make([]forge.CI, 0, len(each))
	for _, state := range each {
		cis = append(cis, forge.CI{State: state})
	}

	return cis
}

func TestAwaitGreenAnswersOnceCIPasses(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIRunning, forge.CIRunning, forge.CIPassed), now: armedAt()}

	// Act
	green, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if err != nil || green.Pull.Number != 9 || world.rounds != 3 {
		t.Errorf("AwaitGreen = #%d, %v after %d reads; want #9 once the third read passed",
			green.Pull.Number, err, world.rounds)
	}
}

func TestAwaitGreenWaitsThroughAFailedRun(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIFailed, forge.CIRunning, forge.CIPassed), now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if err != nil || len(world.heard) == 0 || world.heard[0].Armed != loop.ArmedAwaitingFix {
		t.Errorf("AwaitGreen = %v, heard %+v; want it to wait on a fix, then pass", err, world.heard)
	}
}

func TestAwaitGreenReadsTheHeadAgainEachRound(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIRunning, forge.CIPassed), now: armedAt()}

	// Act
	green, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if err != nil || !slices.Equal(world.heads, []string{"head0", "head1"}) || green.Head != "head1" {
		t.Errorf("AwaitGreen = %+v, %v, read CI at %q; want each round at the branch's head then",
			green, err, world.heads)
	}
}

func TestAwaitGreenGivesUpWithNoCheckReported(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CINone), now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if !errors.Is(err, loop.ErrNoCI) || world.now.Sub(armedAt()) != loop.NoCIPatience {
		t.Errorf("AwaitGreen = %v after %v, want ErrNoCI after %v", err, world.now.Sub(armedAt()), loop.NoCIPatience)
	}
}

func TestAwaitGreenEndsWhenThePullRequestChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	other := openPull()
	other.Number = 10
	world := &awaiting{
		cis: states(forge.CIRunning), pulls: []forge.PullRequest{openPull(), other}, now: armedAt(),
	}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if !errors.Is(err, loop.ErrPullChanged) {
		t.Errorf("AwaitGreen = %v, want ErrPullChanged", err)
	}
}

func TestAwaitGreenEndsWhenItMergesFirst(t *testing.T) {
	t.Parallel()

	// Arrange
	merged := openPull()
	merged.State = forge.StateMerged
	world := &awaiting{cis: states(forge.CIRunning), pulls: []forge.PullRequest{openPull(), merged}, now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if !errors.Is(err, loop.ErrMergedFirst) {
		t.Errorf("AwaitGreen = %v, want ErrMergedFirst", err)
	}
}

func TestAwaitGreenEndsWithItsContext(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	world := &awaiting{cis: states(forge.CIRunning), now: armedAt(), cancel: cancel}

	// Act
	_, err := loop.AwaitGreen(ctx, world.seams(), 9)

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("AwaitGreen = %v, want context.Canceled", err)
	}
}

func TestAwaitGreenWaitsOnThroughAFailedRead(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{
		cis: states(forge.CIRunning, forge.CIPassed), ciErrs: []error{errSeam, nil}, now: armedAt(),
	}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if err != nil || len(world.heard) == 0 || !errors.Is(world.heard[0].Err, errSeam) {
		t.Errorf("AwaitGreen = %v, heard %+v; want the failed read told, then the wait to go on", err, world.heard)
	}
}

func TestAwaitGreenReadsAtOnceBeforeItPauses(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIPassed), now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if err != nil || world.readsAt0 != 1 || world.pauses != 0 {
		t.Errorf("AwaitGreen = %v after %d pauses; want CI read at once and no pause", err, world.pauses)
	}
}

func TestAwaitGreenWaitsOnThroughAFailedReadOfTheBranchOrItsPullRequest(t *testing.T) {
	t.Parallel()

	cases := map[string]*awaiting{
		"the branch":       {cis: states(forge.CIPassed), branchErrs: []error{errSeam, nil}, now: armedAt()},
		"its pull request": {cis: states(forge.CIPassed), pullErrs: []error{errSeam, nil}, now: armedAt()},
	}

	for name, world := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

			// Assert
			if err != nil || len(world.heard) == 0 || !errors.Is(world.heard[0].Err, errSeam) || world.pauses != 1 {
				t.Errorf("AwaitGreen = %v after %d pauses, heard %+v; want the failed read told, then a pass",
					err, world.pauses, world.heard)
			}
		})
	}
}

func TestAwaitGreenNeedsNoListener(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIRunning, forge.CIPassed), now: armedAt()}
	seams := world.seams()
	seams.Heard = nil

	// Act
	_, err := loop.AwaitGreen(t.Context(), seams, 9)
	// Assert
	if err != nil {
		t.Errorf("AwaitGreen with no listener = %v, want it to answer once CI passes", err)
	}
}

func TestAwaitGreenEndsWhenAReadIsCutOffWithItsContext(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	world := &awaiting{cis: states(forge.CIRunning), ciErrs: []error{context.Canceled}, now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(ctx, world.seams(), 9)

	// Assert
	if !errors.Is(err, context.Canceled) || len(world.heard) != 0 {
		t.Errorf("AwaitGreen = %v, heard %+v; want context.Canceled and no failed read told", err, world.heard)
	}
}

func TestAwaitGreenWaitsItsDelayBeforeItsFirstRead(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIPassed), now: armedAt()}
	seams := world.seams()
	seams.Delay = time.Minute

	// Act
	_, err := loop.AwaitGreen(t.Context(), seams, 9)

	// Assert
	if err != nil || world.pauses != 1 || world.readsAt0 != 0 {
		t.Errorf("AwaitGreen = %v after %d pauses, %d reads before the first; want the delay waited, then a read",
			err, world.pauses, world.readsAt0)
	}
}

func TestAwaitGreenEndsWithItsContextDuringItsDelay(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	world := &awaiting{cis: states(forge.CIPassed), now: armedAt()}
	seams := world.seams()
	seams.Delay = time.Minute

	// Act
	_, err := loop.AwaitGreen(ctx, seams, 9)

	// Assert
	if !errors.Is(err, context.Canceled) || world.rounds != 0 {
		t.Errorf("AwaitGreen = %v after %d reads, want context.Canceled and no read", err, world.rounds)
	}
}

func TestAwaitGreenWaitsForAPullRequestJustOpenedToBeListed(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge lists the pull request just opened only from its second find.
	world := &awaiting{cis: states(forge.CIPassed), missing: []bool{true, false}, now: armedAt()}
	seams := world.seams()
	seams.Opened = true

	// Act
	green, err := loop.AwaitGreen(t.Context(), seams, 9)

	// Assert
	if err != nil || green.Pull.Number != 9 || len(world.heard) == 0 ||
		!errors.Is(world.heard[0].Err, loop.ErrNotListedYet) {
		t.Errorf("AwaitGreen = #%d, %v, heard %+v; want #9, after a read that did not list it yet",
			green.Pull.Number, err, world.heard)
	}
}

func TestAwaitGreenEndsWhenAPullRequestJustOpenedIsGoneOnceListed(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIRunning), missing: []bool{false, true}, now: armedAt()}
	seams := world.seams()
	seams.Opened = true

	// Act
	_, err := loop.AwaitGreen(t.Context(), seams, 9)

	// Assert
	if !errors.Is(err, loop.ErrPullChanged) {
		t.Errorf("AwaitGreen = %v, want ErrPullChanged once a find that saw it no longer does", err)
	}
}

func TestAwaitGreenEndsWhenAPullRequestFoundBeforeIsNotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	world := &awaiting{cis: states(forge.CIRunning), missing: []bool{true}, now: armedAt()}

	// Act
	_, err := loop.AwaitGreen(t.Context(), world.seams(), 9)

	// Assert
	if !errors.Is(err, loop.ErrPullChanged) || world.finds != 1 {
		t.Errorf("AwaitGreen = %v after %d finds, want ErrPullChanged on the first", err, world.finds)
	}
}
