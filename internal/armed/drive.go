// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package armed

import (
	"context"
	"fmt"
	"time"
)

// Clock is what Drive waits by: the time patience is counted by, a pause
// that waits under ctx and answers its error once it ends, and the interval
// between reads.
type Clock struct {
	Now      func() time.Time
	Pause    func(ctx context.Context, wait time.Duration) error
	Interval time.Duration
}

// Drive drives the hold just armed under ctx, in the foreground, until it
// ends — posted, posted elsewhere or dropped — reading through seams and
// waiting by clock, telling told of the hold after each step. ctx ending
// while it waits stops it, and answers ctx's error with the stopped hold.
// It starts no goroutine: the clock's Pause is the caller's wait.
func Drive(ctx context.Context, seams Seams, armed Armed, clock Clock, told func(Hold)) (Hold, error) {
	hold, next := armed.Hold, armed.Next

	for next != NextNothing {
		err := clock.before(ctx, next)
		if err != nil {
			stopped, _ := hold.Stop()

			return stopped, err
		}

		if next == NextDeliver {
			hold, next = hold.Delivered(seams.DeliverHeld(ctx, hold, clock.Now))
		} else {
			hold, next = hold.Hear(seams.Read(ctx, hold, clock.Now))
		}

		told(hold)
	}

	return hold, nil
}

// before waits what next asks to wait before it is done: an interval before
// a read made later, and nothing else. A read is not made once ctx has
// ended; a post is, so it ends as loop.Deliver decides.
func (c Clock) before(ctx context.Context, next Next) error {
	if next == NextReadLater {
		return c.Pause(ctx, c.Interval)
	}

	err := ctx.Err()
	if next == NextRead && err != nil {
		return fmt.Errorf("waiting for CI: %w", err)
	}

	return nil
}
