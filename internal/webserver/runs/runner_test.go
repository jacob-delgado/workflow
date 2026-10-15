// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

import (
	"context"
	"sync"
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
	runner := runs.New(given, func(error) {}, &indexWrites)

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
	runner := runs.New(given, func(error) {}, &indexWrites)
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
