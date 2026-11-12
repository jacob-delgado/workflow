// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"fmt"
)

// onceConnected caches a connection once it succeeds, and retries after a
// failure rather than remembering it: a token added, or gh signed in, in another
// terminal is found on the next attempt instead of only on a restart. A connect
// its asker's context ended is a failure like any other, so the next asker
// connects again under its own. One asker connects at a time, and the others
// wait for it only as long as their own contexts last.
func onceConnected[T any](connect func(context.Context) (T, error)) func(context.Context) (T, error) {
	var (
		turn   = newConnecting()
		cached T
		ok     bool
	)

	return func(ctx context.Context) (T, error) {
		err := turn.take(ctx)
		if err != nil {
			var none T

			return none, err
		}
		defer turn.give()

		if ok {
			return cached, nil
		}

		connection, err := connect(ctx)
		if err != nil {
			var none T

			return none, err
		}

		cached, ok = connection, true

		return cached, nil
	}
}

// ask connects with connect under ctx, then asks the connection what question
// does: the one shape every seam over a client found on first use takes,
// Jira's, Taskwarrior's and the forge's alike.
func ask[C, T any](
	ctx context.Context, connect func(context.Context) (C, error), question func(C) (T, error),
) (T, error) {
	connection, err := connect(ctx)
	if err != nil {
		var none T

		return none, err
	}

	return question(connection)
}

// tell connects with connect under ctx, then applies change to the connection.
func tell[C any](ctx context.Context, connect func(context.Context) (C, error), change func(C) error) error {
	connection, err := connect(ctx)
	if err != nil {
		return err
	}

	return change(connection)
}

// connecting is the one connect a client found on first use makes at a
// time: a lock held across the connect, which may run a token command, so an
// asker waiting for it must be able to stop when its own context ends, as a
// mutex would never let it.
type connecting chan struct{}

// newConnecting is a turn no one holds.
func newConnecting() connecting {
	return make(connecting, 1)
}

// take waits for the turn to connect, or until ctx ends. A free turn is taken
// whatever ctx says, so only an asker that did wait on another's connect is
// told it did.
func (c connecting) take(ctx context.Context) error {
	select {
	case c <- struct{}{}:
		return nil
	default:
	}

	select {
	case c <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting on a connection another call is making: %w", ctx.Err())
	}
}

// give hands the turn on.
func (c connecting) give() {
	<-c
}
