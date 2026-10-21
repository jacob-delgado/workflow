// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestARunnerCountsTheUnmanagedHooksItsSeamsFind(t *testing.T) {
	t.Parallel()

	// Arrange
	var indexWrites sync.Mutex

	found := webtest.OldHooks()
	given := runs.Seams{Hooks: seams.Hooks{
		Existing: func(context.Context) ([]hooks.GitHook, bool) { return found, false },
	}}
	runner := runs.New(given, func(error) {}, &indexWrites, func() int { return 1 })

	// Act
	counted := runner.HooksUnmanaged(t.Context())

	// Assert
	if counted != len(found) {
		t.Errorf("HooksUnmanaged() = %d, want %d", counted, len(found))
	}
}

func TestARunHoldsTheIndexItIsGivenWhileItGoes(t *testing.T) {
	t.Parallel()

	// Arrange
	var indexWrites sync.Mutex

	held := newHeldRun("hanging")
	given := runs.Seams{Hooks: seams.Hooks{
		Configured: func(context.Context) (hooks.Configured, error) { return configuredHooks(), nil },
		Run:        func(context.Context, string) (proc.Output, error) { return held.output(), nil },
	}}
	runner := runs.New(given, func(error) {}, &indexWrites, func() int { return 1 })
	recorder := newSyncRecorder()
	done := make(chan struct{})

	go func() {
		defer close(done)

		runner.StartRun(recorder, runRequest(t, preCommitRun))
	}()

	recorder.waitFor(t, "hanging")

	// Act: take the index while the run's program goes
	takenWhileGoing := indexWrites.TryLock()
	if takenWhileGoing {
		indexWrites.Unlock()
	}

	// Assert: the run holds it
	if takenWhileGoing {
		t.Error("the index was free while the run's program went, want it held")
	}

	// Act: end the run, then take the index
	held.output().Stop()
	<-done

	takenAfter := indexWrites.TryLock()

	// Assert: the run freed it
	if !takenAfter {
		t.Error("the index is still held after the run ended, want it freed")
	}
}

func TestARunTellsItLandedBeforeItsLastEvent(t *testing.T) {
	t.Parallel()

	// Arrange
	// landedAs is the number the server counts the run's end as.
	const landedAs = 7

	var (
		indexWrites sync.Mutex
		ended       atomic.Bool
		landings    []string
	)

	output := finished(nil, "lint ok")
	wait := output.Wait
	output.Wait = func() error {
		ended.Store(true)

		return wait()
	}
	given := runs.Seams{Hooks: seams.Hooks{
		Configured: func(context.Context) (hooks.Configured, error) { return configuredHooks(), nil },
		Run:        func(context.Context, string) (proc.Output, error) { return output, nil },
	}}
	recorder := newSyncRecorder()
	landed := func() int {
		if !ended.Load() {
			t.Error("the run said it landed before its program ended")
		}

		landings = append(landings, recorder.body())

		return landedAs
	}
	runner := runs.New(given, func(error) {}, &indexWrites, landed)

	// Act
	runner.StartRun(recorder, runRequest(t, preCommitRun))

	// Assert
	if len(landings) != 1 {
		t.Fatalf("the run said it landed %d times, want once", len(landings))
	}

	if strings.Contains(landings[0], `"succeeded"`) || !strings.Contains(landings[0], "lint ok") {
		t.Errorf("written when it landed: %q; want every line, and not yet the run as it ended", landings[0])
	}

	if !strings.Contains(recorder.body(), `"succeeded"`) || !strings.Contains(recorder.body(), `"write":7`) {
		t.Errorf("the run wrote %q, want it to end with the run succeeded, numbered as its end landed", recorder.body())
	}
}
