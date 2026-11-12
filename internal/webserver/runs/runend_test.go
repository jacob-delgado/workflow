// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

// How a run ends when a stop comes as its program ends.

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
)

// hookRunner is a Runner whose every hook run answers output, over an index
// of its own.
func hookRunner(output func() proc.Output) *runs.Runner {
	given := runs.Seams{Hooks: seams.Hooks{
		Configured: func(context.Context) (hooks.Configured, error) { return configuredHooks(), nil },
		Run:        func(context.Context, string) (proc.Output, error) { return output(), nil },
	}}

	return runs.New(given, func(error) {}, &sync.Mutex{}, func() int { return 1 })
}

func TestAStopThatComesOnceTheProgramPassedLeavesTheRunSucceeded(t *testing.T) {
	t.Parallel()

	// Arrange
	// The program has written all it will and passed; the stop comes as the
	// run reads how it exited.
	waiting, passed := make(chan struct{}), make(chan struct{})
	runner := hookRunner(func() proc.Output {
		output := finished(nil)
		output.Wait = func() error {
			close(waiting)
			<-passed

			return nil
		}

		return output
	})
	recorder, request, done := httptest.NewRecorder(), runRequest(t, preCommitRun), make(chan struct{})

	go func() {
		defer close(done)

		runner.StartRun(recorder, request)
	}()

	<-waiting

	// Act
	_, err := runner.StopRun(t.Context(), api.StopRunRequestObject{})

	close(passed)
	<-done

	// Assert
	if err != nil {
		t.Fatalf("StopRun: %v", err)
	}

	if ended := lastRun(t, runEvents(t, recorder)); ended.State != api.RunStateSucceeded {
		t.Errorf("the run ended %q, %q; want it succeeded: its program passed before the stop reached it",
			ended.State, ended.Outcome)
	}
}
