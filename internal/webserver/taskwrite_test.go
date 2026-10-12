// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The writes on one task whose paths name them by more than their verb.
const (
	annotateWrite = "annotations"
	modifyWrite   = "modify"
)

// taskPath is where a write on the task uuid names is sent: its start, stop,
// done, annotations or modify.
func taskPath(uuid, write string) string {
	return tasksPath + "/" + uuid + "/" + write
}

// taskWrite is one of the task API's writes: where it is sent, and a body it
// takes.
type taskWrite struct {
	path, body string
}

// taskWrites is every write the task API takes, each with a body it accepts.
func taskWrites() map[string]taskWrite {
	return map[string]taskWrite{
		addVerb:      {path: tasksPath, body: addBody},
		"track":      {path: tasksPath + "/track", body: `{"issue_key":"` + testKey + `"}`},
		undoVerb:     {path: tasksPath + "/undo"},
		"sync":       {path: tasksPath + "/sync"},
		startVerb:    {path: taskPath(startedUUID, "start")},
		stopVerb:     {path: taskPath(startedUUID, stopVerb)},
		doneVerb:     {path: taskPath(startedUUID, doneVerb)},
		annotateVerb: {path: taskPath(startedUUID, annotateWrite), body: `{"text":"waiting on review"}`},
		"modify":     {path: taskPath(startedUUID, modifyWrite), body: `{"line":"project:web"}`},
	}
}

func TestStartStopAndDoneAnswerTheListAfterTheWrite(t *testing.T) {
	t.Parallel()

	for _, write := range []string{startVerb, stopVerb, doneVerb} {
		t.Run(write, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()

			// Act
			recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath(plainUUID, write), "")

			// Assert
			list := decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || !list.Available || len(list.Tasks) != 2 || list.Added != nil {
				t.Errorf("answer = %d %+v, want 200 with the pending list and nothing added", recorder.Code, list)
			}

			if call := write + " " + plainUUID; !fake.readAfter(call) {
				t.Errorf("calls = %q, want %q and then the pending list read", fake.asked(), call)
			}
		})
	}
}

func TestAnnotateAndModifySendTheirText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		write, body, wantCall string
	}{
		annotateVerb: {
			write: annotateWrite, body: `{"text":"waiting on review"}`,
			wantCall: "annotate " + startedUUID + " waiting on review",
		},
		"modify": {
			write: modifyWrite, body: `{"line":"project:web due:friday"}`,
			wantCall: "modify " + startedUUID + " project:web due:friday",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			handler := serve(t, tasksDeps(fake), config.Default())

			// Act
			recorder := send(t, handler, http.MethodPost, taskPath(startedUUID, tt.write), tt.body)

			// Assert
			if recorder.Code != http.StatusOK || !fake.readAfter(tt.wantCall) {
				t.Errorf("answer = %d, calls = %q, want 200 after %q", recorder.Code, fake.asked(), tt.wantCall)
			}
		})
	}
}

func TestAWriteTaskwarriorDeclinesIs409(t *testing.T) {
	t.Parallel()

	// A start reopens a finished task rather than declining it, so its decline
	// has words of its own.
	inThatState := "the task is already in that state, or is no longer pending"
	cases := map[string]string{
		startVerb: "the task is already started, or no such task exists",
		stopVerb:  inThatState,
		doneVerb:  inThatState,
	}

	for write, wantDetail := range cases {
		t.Run(write, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail[write] = taskwarrior.ErrNothingChanged
			handler := serve(t, tasksDeps(fake), config.Default())

			// Act
			recorder := send(t, handler, http.MethodPost, taskPath(startedUUID, write), "")

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeConflict || failure.Detail != wantDetail {
				t.Errorf("answer = %d %+v, want 409 conflict saying %q", recorder.Code, failure, wantDetail)
			}
		})
	}
}

func TestAWriteWhoseListTimesOutSaysWhyTheListIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.fail["pending"] = fmt.Errorf("%s: %w after 10s", dataPath, proc.ErrTimedOut)

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath(plainUUID, "start"), "")

	// Assert
	list := decode[api.TaskList](t, recorder)
	if fake.count("start") != 1 || recorder.Code != http.StatusOK ||
		!strings.HasSuffix(list.Reason, "Taskwarrior did not answer in time") {
		t.Errorf("answer = %d %+v, calls = %q, want the start made, answered 200 with the read's reason",
			recorder.Code, list, fake.asked())
	}
}

func TestAnAnswerThatIsNotTaskwarriorsJSONIsHandedToUnexpected(t *testing.T) {
	t.Parallel()

	// Arrange
	var heard []error

	fake := fakeTaskwarrior()
	fake.fail["start"] = fmt.Errorf("%w: reading the export", taskwarrior.ErrBadOutput)
	deps := tasksDeps(fake)
	deps.Unexpected = func(_ config.Config, err error) { heard = append(heard, err) }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, taskPath(startedUUID, "start"), "")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusInternalServerError || failure.Code != api.ProblemCodeInternal {
		t.Errorf("answer = %d %+v, want the 500 internal problem", recorder.Code, failure)
	}

	if len(heard) != 1 || !errors.Is(heard[0], taskwarrior.ErrBadOutput) {
		t.Errorf("Unexpected heard %v, want the bad answer once", heard)
	}
}

func TestAMalformedUUIDIs400(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, taskPath("not-a-uuid", "start"), "")

	// Assert
	if recorder.Code != http.StatusBadRequest || len(fake.asked()) != 0 {
		t.Errorf("answer = %d, calls = %q, want 400 and Taskwarrior never asked", recorder.Code, fake.asked())
	}
}

func TestAnnotateAndModifyRequireText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		write, body string
	}{
		"an empty annotation":  {write: annotateWrite, body: `{"text":""}`},
		"a blank annotation":   {write: annotateWrite, body: `{"text":"   "}`},
		"an empty modify line": {write: modifyWrite, body: `{"line":""}`},
		"a blank modify line":  {write: modifyWrite, body: `{"line":" "}`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()

			// Act
			recorder := send(t, serve(t, tasksDeps(fake), config.Default()),
				http.MethodPost, taskPath(startedUUID, tt.write), tt.body)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity || len(fake.asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 422 before Taskwarrior is asked", recorder.Code, fake.asked())
			}
		})
	}
}

func TestATaskWriteThatLandedIsAnsweredWhenTheListCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	for name, write := range taskWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail["pending"] = fmt.Errorf("reading %s: %w", dataPath, errSeam)

			// Act
			recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, write.path, write.body)

			// Assert
			list := decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || list.Available || !strings.Contains(list.Reason, "could not be read again") {
				t.Errorf("answer = %d %+v, want 200 with the list marked unavailable, saying it could not be read again",
					recorder.Code, list)
			}

			if strings.Contains(recorder.Body.String(), dataPath) {
				t.Errorf("body = %q, names the data directory", recorder.Body.String())
			}
		})
	}
}

func TestAnAddThatLandedNamesItsTaskWhenTheListCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.fail["pending"] = errSeam

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, tasksPath, addBody)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || list.Added == nil || *list.Added != addedUUID {
		t.Errorf("answer = %d, added = %v; want 200 naming the added task %s", recorder.Code, list.Added, addedUUID)
	}
}

func TestATaskWriteWithoutItsSeamIs422(t *testing.T) {
	t.Parallel()

	for name, write := range taskWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Tasks = seams.Tasks{}

			// Act
			recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, write.path, write.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != "Taskwarrior is not available" {
				t.Errorf("answer = %d %+v, want 422 saying Taskwarrior is not available", recorder.Code, failure)
			}
		})
	}
}

func TestTaskWritesAreRefusedUnderDryRun(t *testing.T) {
	t.Parallel()

	for name, write := range taskWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			dryRun := webserver.Info{Version: testVersion, DryRun: true}
			handler := serveWith(t, tasksDeps(fake), config.Default(), dryRun)

			// Act
			recorder := send(t, handler, http.MethodPost, write.path, write.body)

			// Assert
			if recorder.Code != http.StatusForbidden || len(fake.asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 403 and nothing asked of Taskwarrior", recorder.Code, fake.asked())
			}
		})
	}
}

func TestTaskWritesRefuseACrossOriginRequest(t *testing.T) {
	t.Parallel()

	for name, write := range taskWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			handler := serve(t, tasksDeps(fake), config.Default())
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, write.path, strings.NewReader(write.body))
			request.Host = loopbackHost
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://evil")

			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != http.StatusForbidden || len(fake.asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 403 and nothing asked of Taskwarrior", recorder.Code, fake.asked())
			}
		})
	}
}
