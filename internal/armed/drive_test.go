// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// driveInterval is how often the driven holds read.
const driveInterval = time.Minute

// fakeClock tells a time that moves only as Drive pauses on it.
type fakeClock struct {
	now    time.Time
	pauses int
}

// clock is the fake's Clock, pausing an interval at a time.
func (c *fakeClock) clock() armed.Clock {
	return armed.Clock{
		Now: func() time.Time { return c.now },
		Pause: func(ctx context.Context, wait time.Duration) error {
			if ctx.Err() != nil {
				return fmt.Errorf("pausing: %w", ctx.Err())
			}

			c.now, c.pauses = c.now.Add(wait), c.pauses+1

			return nil
		},
		Interval: driveInterval,
	}
}

// armedNow is a hold armed with arm at armedAt, asking to be read.
func armedNow(t *testing.T, arm armed.Arming) armed.Armed {
	t.Helper()

	got, err := armed.Hold{}.Arm(armedAt(), arm)
	if err != nil {
		t.Fatalf("arming: %v", err)
	}

	return got
}

// stages notes the stage of each hold it is told of.
type stages []armed.Stage

func (s *stages) told(hold armed.Hold) {
	*s = append(*s, hold.Stage())
}

func TestDriveWaitsForCIToPassThenPostsOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	var texts []string

	scripted := &scriptedForge{
		finds: []forge.PullRequest{heldPull()},
		cis:   []forge.CI{{State: forge.CIRunning}, {State: forge.CIFailed}, {State: forge.CIPassed}},
	}
	clock := &fakeClock{now: armedAt()}

	var told stages

	// Act
	got, err := armed.Drive(t.Context(), scripted.seams(postsTo(&texts)), armedNow(t, arming()), clock.clock(),
		told.told)

	// Assert
	if err != nil || got.Stage() != armed.StagePosted || len(texts) != 1 {
		t.Fatalf("Drive = %s (%v), %v, posting %d times; want it posted once",
			stageName(got.Stage()), got.Reason(), err, len(texts))
	}

	want := stages{armed.StageRunning, armed.StageAwaitingFix, armed.StageDue, armed.StagePosted}
	if fmt.Sprint(told) != fmt.Sprint(want) || clock.pauses != 2 {
		t.Errorf("told %v after %d pauses, want %v after 2: a read at once, then one an interval apart",
			told, clock.pauses, want)
	}
}

func TestDriveGivesUpOnCIItCannotReadForItsPatience(t *testing.T) {
	t.Parallel()

	// Arrange
	// C38: a CI read that always fails ends the wait, rather than polling for
	// as long as the command runs.
	scripted := &scriptedForge{finds: []forge.PullRequest{heldPull()}, ciErr: errSeam}
	clock := &fakeClock{now: armedAt()}

	// Act
	got, err := armed.Drive(t.Context(), scripted.seams(nil), armedNow(t, arming()), clock.clock(), new(stages).told)

	// Assert
	if err != nil || got.Stage() != armed.StageDropped || !errors.Is(got.Reason(), armed.ErrUnheard) ||
		!errors.Is(got.Reason(), errSeam) {
		t.Errorf("Drive = %s (%v), %v; want it dropped, unheard", stageName(got.Stage()), got.Reason(), err)
	}

	if waited := clock.now.Sub(armedAt()); waited != armed.UnheardPatience {
		t.Errorf("Drive gave up after %v, want %v", waited, armed.UnheardPatience)
	}
}

func TestDriveGivesUpOnAPullRequestNeverListed(t *testing.T) {
	t.Parallel()

	// Arrange
	// C37: one just opened that the branch armed never lists is given up on.
	scripted := &scriptedForge{finds: []forge.PullRequest{{}}}
	clock := &fakeClock{now: armedAt()}

	// Act
	got, err := armed.Drive(t.Context(), scripted.seams(nil), armedNow(t, opening()), clock.clock(), new(stages).told)

	// Assert
	if err != nil || got.Stage() != armed.StageDropped || !errors.Is(got.Reason(), armed.ErrNotListedYet) {
		t.Errorf("Drive = %s (%v), %v; want it dropped, never listed", stageName(got.Stage()), got.Reason(), err)
	}
}

func TestDriveGivesUpOnAPullRequestClosedAfterTheGateRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// C39: CI passed, a check failed by the gate's own read, then the pull
	// request was closed, which a find skips.
	scripted := &scriptedForge{
		finds: []forge.PullRequest{heldPull(), {}},
		cis:   []forge.CI{{State: forge.CIPassed}, {State: forge.CIFailed}},
	}
	clock := &fakeClock{now: armedAt()}

	// Act
	got, err := armed.Drive(t.Context(), scripted.seams(postsTo(new([]string))), armedNow(t, arming()),
		clock.clock(), new(stages).told)

	// Assert
	if err != nil || got.Stage() != armed.StageDropped || !errors.Is(got.Reason(), armed.ErrPullChanged) {
		t.Errorf("Drive = %s (%v), %v; want it dropped, the pull request changed",
			stageName(got.Stage()), got.Reason(), err)
	}
}

func TestDriveStopsWhenItsContextEndsAsItWaits(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	scripted := &scriptedForge{finds: []forge.PullRequest{heldPull()}, cis: []forge.CI{{State: forge.CIRunning}}}
	seams := scripted.seams(nil)
	seams.CheckCI = func(ctx context.Context, pull forge.PullRequest, head string) (forge.CI, error) {
		cancel()

		return scripted.checkCI(ctx, pull, head)
	}

	// Act
	got, err := armed.Drive(ctx, seams, armedNow(t, arming()), (&fakeClock{now: armedAt()}).clock(),
		new(stages).told)

	// Assert
	if !errors.Is(err, context.Canceled) || got.Stage() != armed.StageStopped {
		t.Errorf("Drive = %s, %v; want it stopped with its context's error", stageName(got.Stage()), err)
	}
}

func TestDriveReadsNothingOnceItsContextHasEnded(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	scripted := &scriptedForge{finds: []forge.PullRequest{heldPull()}}

	// Act
	got, err := armed.Drive(ctx, scripted.seams(nil), armedNow(t, arming()), (&fakeClock{now: armedAt()}).clock(),
		new(stages).told)

	// Assert
	if !errors.Is(err, context.Canceled) || got.Stage() != armed.StageStopped || len(scripted.branches) != 0 {
		t.Errorf("Drive = %s, %v, finding %d times; want it stopped unread", stageName(got.Stage()), err,
			len(scripted.branches))
	}
}

func TestDriveEndsAtOnceOnAHoldThatAsksForNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	hold := running(t)

	// Act
	got, err := armed.Drive(t.Context(), armed.Seams{}, armed.Armed{Hold: hold, Next: armed.NextNothing},
		(&fakeClock{now: armedAt()}).clock(), new(stages).told)

	// Assert
	if err != nil || got.Stage() != armed.StageRunning {
		t.Errorf("Drive = %s, %v; want the hold back as it was", stageName(got.Stage()), err)
	}
}
