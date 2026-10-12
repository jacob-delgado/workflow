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
	"github.com/jacob-delgado/workflow/internal/proc"
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

// printingOutput is a program that prints a line and exits once the line is
// read, as one blocked on a full pipe does: left unread, it never exits, and
// Wait waits for it.
func printingOutput(t *testing.T) proc.Output {
	t.Helper()

	lines, exited := make(chan string), make(chan struct{})

	go func() {
		defer close(exited)
		defer close(lines)

		select {
		case lines <- "[main 1a2b3c4] fix(login): retry once":
		case <-t.Context().Done():
		}
	}()

	return proc.Output{
		Lines: lines,
		Wait: func() error {
			<-exited

			return nil
		},
		Stop: func() {},
	}
}

// A quit or a switch ends the session mid-run, and the interface reads no
// more of the run's output, so nothing calls Wait as it would: the write
// still counts until its program exits, and no longer.
func TestAStreamedWriteTheInterfaceLeftCountsUntilItsProgramExits(t *testing.T) {
	t.Parallel()

	for _, path := range streamedWrites() {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			// Arrange
			flow := writeFlows()[path]
			start := flow.start(t)
			seam := seamAt(t, &start.deps, path)
			seam.Set(reflect.MakeFunc(seam.Type(), func([]reflect.Value) []reflect.Value {
				return []reflect.Value{reflect.ValueOf(printingOutput(t)), reflect.Zero(seam.Type().Out(1))}
			}))

			inFlight := lifetime.NewInFlight(0)
			program := lifetime.Counting(t.Context(), inFlight)
			session, quit := context.WithCancel(t.Context())
			model := tui.New(start.cfg, start.loadErr, start.deps).WithLifetime(program).WithContext(session)
			model = sized(t, model, 140, 50)
			model = drain(t, model, model.Init())
			startRun(t, model, flow.keys)

			// Act
			quit()

			// Assert
			settling, cancel := context.WithTimeout(t.Context(), quitPatience)
			defer cancel()

			unfinished, err := inFlight.Settled(settling)
			if err != nil {
				t.Errorf("%s still counts in flight as %q once the interface quit and its program could exit, "+
					"want it counted only until the program exits", path, unfinished)
			}
		})
	}
}
