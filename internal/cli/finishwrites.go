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

	"github.com/jacob-delgado/workflow/internal/lifetime"
)

// writesSettle is how long no write may have been in flight before the
// writes a quit waits for are taken to have landed: a write that follows
// another in one step, as a post's record follows the post, begins just as
// the first lands.
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
	if pending == 0 {
		return nil
	}

	interrupted := cmd.Context().Err()
	if interrupted != nil {
		return fmt.Errorf("interrupted with %s in flight: %w", writesCounted(pending), interrupted)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "finishing %s…\n", writesCounted(pending))

	waiting, cancel := context.WithTimeoutCause(cmd.Context(), timeout, errWritesUnfinished)
	defer cancel()

	unfinished, err := writes.Settled(waiting)

	// A signal is checked first: the writes it ends land at once, and are not
	// what the wait was for.
	interrupted = cmd.Context().Err()

	switch {
	case interrupted != nil:
		return fmt.Errorf("interrupted while finishing %s: %w", writesCounted(pending), interrupted)
	case err == nil:
		return nil
	default:
		return fmt.Errorf("%w after %s: %s", errWritesUnfinished, timeout, strings.Join(unfinished, ", "))
	}
}

// writesCounted is count writes, in words: "1 write", "2 writes".
func writesCounted(count int) string {
	if count == 1 {
		return "1 write"
	}

	return strconv.Itoa(count) + " writes"
}
