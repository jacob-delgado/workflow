// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
)

// InFlight is the work begun under one lifetime that has not yet ended, each
// piece named for what it does: what a process waits for before it exits, so
// it does not cut that work off.
type InFlight struct {
	settle time.Duration

	lock sync.Mutex
	open map[int]string
	// begun is how many pieces have begun, which numbers the next.
	begun int
	// idle is closed while nothing is in flight.
	idle chan struct{}
}

// NewInFlight is work in flight, none yet, that Settled takes to have settled
// once nothing has been in flight for settle: a piece that follows another in
// one step begins just as the first ends, and is waited for too.
func NewInFlight(settle time.Duration) *InFlight {
	idle := make(chan struct{})
	close(idle)

	return &InFlight{settle: settle, open: map[int]string{}, idle: idle}
}

// inFlightKey is where a context carries the InFlight its work is counted in.
type inFlightKey struct{}

// Counting is ctx carrying inFlight, which work begun under it is counted in.
func Counting(ctx context.Context, inFlight *InFlight) context.Context {
	return context.WithValue(ctx, inFlightKey{}, inFlight)
}

// CountedIn is the InFlight ctx carries, or, where it carries none, one that
// nothing waits on.
func CountedIn(ctx context.Context) *InFlight {
	inFlight, carried := ctx.Value(inFlightKey{}).(*InFlight)
	if !carried {
		return NewInFlight(0)
	}

	return inFlight
}

// Begin counts a piece of work named name in flight until the function it
// returns is called; calling that again does nothing.
func (f *InFlight) Begin(name string) func() {
	f.lock.Lock()
	defer f.lock.Unlock()

	if len(f.open) == 0 {
		f.idle = make(chan struct{})
	}

	piece := f.begun
	f.begun++
	f.open[piece] = name

	return sync.OnceFunc(func() { f.end(piece) })
}

// Names are the work in flight now, each piece by its name, in the order it
// began.
func (f *InFlight) Names() []string {
	f.lock.Lock()
	defer f.lock.Unlock()

	names := make([]string, 0, len(f.open))
	for _, piece := range slices.Sorted(maps.Keys(f.open)) {
		names = append(names, f.open[piece])
	}

	return names
}

// Settled returns once nothing has been in flight for the settle NewInFlight
// was given. When ctx ends first, it answers the names of the work still in
// flight then, with ctx's cause, or nothing at all when none is.
func (f *InFlight) Settled(ctx context.Context) ([]string, error) {
	for {
		begun, err := f.idleAfter(ctx)
		if err == nil {
			var quiet bool

			quiet, err = f.quietFor(ctx, begun)
			if quiet {
				return nil, nil
			}
		}

		if err != nil {
			return f.cutShort(err)
		}
	}
}

// idleAfter returns once nothing is in flight, with how many pieces had begun
// by then, or with ctx's cause once ctx ends first.
func (f *InFlight) idleAfter(ctx context.Context) (int, error) {
	for {
		f.lock.Lock()
		idle, begun, open := f.idle, f.begun, len(f.open)
		f.lock.Unlock()

		if open == 0 {
			return begun, nil
		}

		select {
		case <-idle:
		case <-ctx.Done():
			return 0, fmt.Errorf("waiting for work in flight: %w", context.Cause(ctx))
		}
	}
}

// quietFor reports, once the settle has passed, whether nothing has begun
// since begun pieces had, and so nothing is in flight, or answers ctx's cause
// once ctx ends first.
func (f *InFlight) quietFor(ctx context.Context, begun int) (bool, error) {
	timer := time.NewTimer(f.settle)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
		return false, fmt.Errorf("waiting for work in flight to settle: %w", context.Cause(ctx))
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	return f.begun == begun, nil
}

// cutShort is the work a wait that ended with cause leaves in flight, by
// name, with cause, or nothing at all when none is left.
func (f *InFlight) cutShort(cause error) ([]string, error) {
	unfinished := f.Names()
	if len(unfinished) == 0 {
		return nil, nil
	}

	return unfinished, cause
}

// end counts piece out of flight.
func (f *InFlight) end(piece int) {
	f.lock.Lock()
	defer f.lock.Unlock()

	delete(f.open, piece)

	if len(f.open) == 0 {
		close(f.idle)
	}
}
