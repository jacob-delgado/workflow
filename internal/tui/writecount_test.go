// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

// Every write the interface sends counts in flight, under a name that says
// what it does, in the work its program's lifetime counts, so the command
// line can wait for it on a quit and name what it gave up on.

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/lifetime"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestEveryWriteCountsInFlightUnderItsName(t *testing.T) {
	t.Parallel()

	for path, flow := range writeFlows() {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			start := flow.start(t)
			held := newHold(t)
			asked := holdWrite(t, held, &start.deps, path)
			inFlight := lifetime.NewInFlight(0)
			program := lifetime.Counting(t.Context(), inFlight)
			model := sized(t, tui.New(start.cfg, start.loadErr, start.deps).WithLifetime(program), 140, 50)

			// Act
			holding(t, held, drainPast(t, held, model, model.Init()), flow.keys...)

			// Assert
			held.requireReached(t)
			<-asked

			want := []string{flow.name}
			if got := inFlight.Names(); !slices.Equal(got, want) {
				t.Errorf("%s in flight counts as %q, want %q", path, got, want)
			}
		})
	}
}

func TestAStreamedWriteCountsInFlightUntilItsProgramExits(t *testing.T) {
	t.Parallel()

	for _, path := range streamedWrites() {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			flow := writeFlows()[path]
			start := flow.start(t)
			seam := seamAt(t, &start.deps, path)
			seam.Set(reflect.MakeFunc(seam.Type(), func([]reflect.Value) []reflect.Value {
				return []reflect.Value{reflect.ValueOf(runningOutput()), reflect.Zero(seam.Type().Out(1))}
			}))

			inFlight := lifetime.NewInFlight(0)
			program := lifetime.Counting(context.Background(), inFlight)
			model := sized(t, tui.New(start.cfg, start.loadErr, start.deps).WithLifetime(program), 140, 50)
			model = drain(t, model, model.Init())

			// Act: start the run
			running, readToTheEnd := startRun(t, model, flow.keys)

			// Assert: it counts in flight while its program runs
			if got := inFlight.Names(); len(got) != 1 {
				t.Errorf("%s counts in flight as %q while its program runs, want it alone", path, got)
			}

			// Act: stop it, and read its output to the end
			typing(t, running, "s")
			readToTheEnd()

			// Assert: it counts no longer
			if got := inFlight.Names(); len(got) != 0 {
				t.Errorf("%s still counts in flight as %q once its program exited", path, got)
			}
		})
	}
}
