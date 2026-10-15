// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/lifetime"
)

// writesSettle is how long no write may have been in flight before the
// writes a quit waits for are taken to have landed: a write that follows
// another in one step, as a post's record follows the post, begins just as
// the first lands, and a write a command sends begins just after the command
// starts. Every quit settles once, so a quit that lands in such a gap, with
// nothing counted, still waits for the write that begins after it.
const writesSettle = 100 * time.Millisecond

// errWritesUnfinished is a quit that gave up waiting for the writes the
// interfaces left in flight.
var errWritesUnfinished = errors.New("gave up on writes still in flight")

// finishWrites waits, once the last interface has quit, for the writes the
// interfaces left in flight to land, saying on stderr how many it is
// finishing: the process would otherwise exit under them. It gives up after
// timeout, naming each write still in flight, and stops at once when cmd's
// context ends, as a second Ctrl+C or a signal ends it, which ends every
// write's context too. A signal that ended the interface itself, before any
// wait, begins none.
func finishWrites(cmd *cobra.Command, writes *lifetime.InFlight, timeout time.Duration) error {
	pending := len(writes.Names())

	interrupted := cmd.Context().Err()
	if interrupted != nil {
		return interruptedWith(pending, interrupted)
	}

	if pending > 0 {
		cmdkit.NotesOf(cmd).Note("finishing " + writesCounted(pending) + "…")
	}

	return settleWrites(cmd.Context(), writes, pending, timeout)
}

// settleWrites waits under ctx, up to timeout, for writes to settle once
// pending had been announced as in flight, none at all when pending is zero.
func settleWrites(ctx context.Context, writes *lifetime.InFlight, pending int, timeout time.Duration) error {
	waiting, cancel := context.WithTimeoutCause(ctx, timeout, errWritesUnfinished)
	defer cancel()

	unfinished, err := writes.Settled(waiting)

	// A signal is checked first: the writes it ends land at once, and are not
	// what the wait was for.
	interrupted := ctx.Err()

	switch {
	case interrupted != nil && pending > 0:
		return fmt.Errorf("interrupted while finishing %s: %w", writesCounted(pending), interrupted)
	case interrupted != nil:
		return interruptedWith(len(unfinished), interrupted)
	case err == nil:
		return nil
	default:
		return fmt.Errorf("%w after %s: %s", errWritesUnfinished, timeout, strings.Join(unfinished, ", "))
	}
}

// interruptedWith is a signal's interruption with count writes in flight, no
// wait for them announced, or nothing when none is.
func interruptedWith(count int, interrupted error) error {
	if count == 0 {
		return nil
	}

	return fmt.Errorf("interrupted with %s in flight: %w", writesCounted(count), interrupted)
}

// writesCounted is count writes, in words: "1 write", "2 writes".
func writesCounted(count int) string {
	if count == 1 {
		return "1 write"
	}

	return strconv.Itoa(count) + " writes"
}
