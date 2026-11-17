// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package grace_test

import (
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc/grace"
)

// patience is the grace every watch here starts with.
const patience = 2 * time.Second

// startedAt is when every watch here is first moved.
func startedAt() time.Time {
	return time.Date(2026, time.November, 17, 9, 0, 0, 0, time.UTC)
}

// later is startedAt moved on by elapsed.
func later(elapsed time.Duration) time.Time {
	return startedAt().Add(elapsed)
}

// reading is a watch whose reader waits on the pipe while the program runs.
func reading() grace.Watch {
	return grace.Start(patience).OnPipe(startedAt())
}

// counting is a watch counting down from startedAt: the program exited while
// its reader waited on the pipe.
func counting() grace.Watch {
	return reading().Reaped(startedAt())
}

// paused is a watch whose count stopped a second in, the reader having left
// the pipe to hand a line on.
func paused() grace.Watch {
	return counting().OffPipe(later(time.Second))
}

func TestAWatchStartsSendingWithItsWholePatience(t *testing.T) {
	t.Parallel()

	// Act
	watch := grace.Start(patience)

	// Assert
	if watch.Stage() != grace.StageSending || watch.Left(later(time.Hour)) != patience {
		t.Errorf("Start = %v with %v left; want sending with %v left",
			watch.Stage(), watch.Left(later(time.Hour)), patience)
	}
}

func TestAPauseCountsOnlyTheTimeSpentWaitingOnThePipe(t *testing.T) {
	t.Parallel()

	// Arrange
	watch := paused()

	// Act
	watch = watch.OnPipe(later(time.Hour))

	// Assert
	if got := watch.Left(later(time.Hour)); got != time.Second {
		t.Errorf("Left once the reader is back on the pipe = %v; want 1s, the hour away uncounted", got)
	}
}

func TestACountGoesOnFromWhereItWasLeft(t *testing.T) {
	t.Parallel()

	// Arrange
	watch := paused().OnPipe(later(time.Hour))

	// Act
	left := watch.Left(later(time.Hour + 500*time.Millisecond))

	// Assert
	if left != 500*time.Millisecond {
		t.Errorf("Left half a second after counting again = %v; want 500ms", left)
	}
}

func TestEachMoveFromEachStage(t *testing.T) {
	t.Parallel()

	sending := func() grace.Watch { return grace.Start(patience) }
	cases := []struct {
		name string
		from func() grace.Watch
		move func(grace.Watch, time.Time) grace.Watch
		want grace.Stage
		left time.Duration
	}{
		{"on the pipe while sending", sending, grace.Watch.OnPipe, grace.StageReading, patience},
		{"off the pipe while sending", sending, grace.Watch.OffPipe, grace.StageSending, patience},
		{"reaped while sending", sending, grace.Watch.Reaped, grace.StagePaused, patience},
		{"on the pipe while reading", reading, grace.Watch.OnPipe, grace.StageReading, patience},
		{"off the pipe while reading", reading, grace.Watch.OffPipe, grace.StageSending, patience},
		{"reaped while reading", reading, grace.Watch.Reaped, grace.StageCounting, 1500 * time.Millisecond},
		{"on the pipe while counting", counting, grace.Watch.OnPipe, grace.StageCounting, 500 * time.Millisecond},
		{"off the pipe while counting", counting, grace.Watch.OffPipe, grace.StagePaused, time.Second},
		{"reaped while counting", counting, grace.Watch.Reaped, grace.StageCounting, 500 * time.Millisecond},
		{"on the pipe while paused", paused, grace.Watch.OnPipe, grace.StageCounting, 500 * time.Millisecond},
		{"off the pipe while paused", paused, grace.Watch.OffPipe, grace.StagePaused, time.Second},
		{"reaped while paused", paused, grace.Watch.Reaped, grace.StagePaused, time.Second},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			watch := test.from()

			// Act
			moved := test.move(watch, later(time.Second))

			// Assert
			got := moved.Left(later(1500 * time.Millisecond))
			if moved.Stage() != test.want || got != test.left {
				t.Errorf("%s = %v with %v left half a second on; want %v with %v left",
					test.name, moved.Stage(), got, test.want, test.left)
			}
		})
	}
}

func TestACountNeverGoesBelowNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	watch := counting()

	// Act
	left := watch.Left(later(time.Hour))

	// Assert
	if left != 0 {
		t.Errorf("Left an hour into a two-second count = %v; want 0", left)
	}
}
