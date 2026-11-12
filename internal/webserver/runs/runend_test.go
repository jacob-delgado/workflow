// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package runs_test

// How a run ends when a stop, or the next run, comes as its program ends.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/webserver/runs"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
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

// endWatcher is a page reading a run's stream that, as the run's last event is
// written, does atEnd first: what a page does the moment it reads that the run
// ended, before the run has let go of anything.
type endWatcher struct {
	*httptest.ResponseRecorder

	atEnd func()
	once  sync.Once
}

func (e *endWatcher) Write(part []byte) (int, error) {
	if strings.Contains(string(part), `"write":`) {
		e.once.Do(e.atEnd)
	}

	return e.ResponseRecorder.Write(part) //nolint:wrapcheck // a recorder's write is the write the run made
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

func TestAStopOnceTheRunHasEndedFindsNoRunGoing(t *testing.T) {
	t.Parallel()

	// Arrange
	var stopped api.StopRunResponseObject

	runner := hookRunner(passing)
	recorder := &endWatcher{ResponseRecorder: httptest.NewRecorder()}
	recorder.atEnd = func() { stopped, _ = runner.StopRun(context.Background(), api.StopRunRequestObject{}) }

	// Act
	runner.StartRun(recorder, runRequest(t, preCommitRun))

	// Assert
	if _, refused := stopped.(api.StopRun404ApplicationProblemPlusJSONResponse); !refused {
		t.Errorf("a stop sent once the run's end was read answered %T, want 404: no run is going", stopped)
	}
}

func TestTheNextRunAskedForOnceARunHasEndedIsNotRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	// The page starts the next run the moment it reads the first one's end.
	runner := hookRunner(passing)
	next, nextAnswered := httptest.NewRecorder(), make(chan struct{})
	nextRequest := runRequest(t, prePushRun)
	recorder := &endWatcher{ResponseRecorder: httptest.NewRecorder()}
	recorder.atEnd = func() {
		go func() {
			defer close(nextAnswered)

			runner.StartRun(next, nextRequest)
		}()

		claimedOrAnswered(t, runner, nextAnswered)
	}

	// Act
	runner.StartRun(recorder, runRequest(t, preCommitRun))
	<-nextAnswered

	// Assert
	if next.Code != http.StatusOK {
		t.Fatalf("the next run answered %d once the first had ended, want it run: %s", next.Code, next.Body.String())
	}

	if ended := lastRun(t, runEvents(t, next)); ended.Title != webtest.PrePushHook {
		t.Errorf("the next run ended as %q, want the %s run asked for", ended.Title, webtest.PrePushHook)
	}
}

// claimedOrAnswered waits until the pre-push run has claimed the slot, which
// the stream then shows, or has been answered, whichever comes first.
func claimedOrAnswered(t *testing.T, runner *runs.Runner, answered <-chan struct{}) {
	t.Helper()

	failsafe := time.After(writeWait)

	for {
		if shown := runner.RunShown(); shown != nil && shown.Title == webtest.PrePushHook {
			return
		}

		select {
		case <-answered:
			return
		case <-failsafe:
			t.Error("the next run neither claimed the slot nor was answered")

			return
		case <-time.After(time.Millisecond):
		}
	}
}
