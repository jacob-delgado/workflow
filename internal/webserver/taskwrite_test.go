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
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The writes on one task whose paths name them by more than their verb.
const (
	annotateWrite = "annotations"
	modifyWrite   = "modify"
)

// taskPath is where a write on the task uuid names is sent: its start, stop,
// done, annotations or modify.
func taskPath(uuid, write string) string {
	return webtest.TasksPath + "/" + uuid + "/" + write
}

// taskWrite is one of the task API's writes: where it is sent, and a body it
// takes.
type taskWrite struct {
	path, body string
}

// taskWrites is every write the task API takes, each with a body it accepts.
func taskWrites() map[string]taskWrite {
	return map[string]taskWrite{
		addVerb:      {path: webtest.TasksPath, body: addBody},
		"track":      {path: webtest.TasksPath + "/track", body: `{"issue_key":"` + webtest.TestKey + `"}`},
		undoVerb:     {path: webtest.TasksPath + "/undo"},
		"sync":       {path: webtest.TasksPath + "/sync"},
		startVerb:    {path: taskPath(webtest.StartedUUID, "start")},
		stopVerb:     {path: taskPath(webtest.StartedUUID, stopVerb)},
		doneVerb:     {path: taskPath(webtest.StartedUUID, doneVerb)},
		annotateVerb: {path: taskPath(webtest.StartedUUID, annotateWrite), body: `{"text":"waiting on review"}`},
		"modify":     {path: taskPath(webtest.StartedUUID, modifyWrite), body: `{"line":"project:web"}`},
	}
}

func TestStartStopAndDoneAnswerTheListAfterTheWrite(t *testing.T) {
	t.Parallel()

	for _, write := range []string{startVerb, stopVerb, doneVerb} {
		t.Run(write, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()), http.MethodPost,
				taskPath(webtest.PlainUUID, write), "")

			// Assert
			list := webtest.Decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || !list.Available || len(list.Tasks) != 2 || list.Added != nil {
				t.Errorf("answer = %d %+v, want 200 with the pending list and nothing added", recorder.Code, list)
			}

			if call := write + " " + webtest.PlainUUID; !fake.ReadAfter(call) {
				t.Errorf("calls = %q, want %q and then the pending list read", fake.Asked(), call)
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
			wantCall: "annotate " + webtest.StartedUUID + " waiting on review",
		},
		"modify": {
			write: modifyWrite, body: `{"line":"project:web due:friday"}`,
			wantCall: "modify " + webtest.StartedUUID + " project:web due:friday",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			handler := webtest.Serve(t, tasksDeps(fake), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, taskPath(webtest.StartedUUID, tt.write), tt.body)

			// Assert
			if recorder.Code != http.StatusOK || !fake.ReadAfter(tt.wantCall) {
				t.Errorf("answer = %d, calls = %q, want 200 after %q", recorder.Code, fake.Asked(), tt.wantCall)
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
			fake := webtest.FakeTaskwarrior()
			fake.Fail[write] = taskwarrior.ErrNothingChanged
			handler := webtest.Serve(t, tasksDeps(fake), config.Default())

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, taskPath(webtest.StartedUUID, write), "")

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeConflict || failure.Detail != wantDetail {
				t.Errorf("answer = %d %+v, want 409 conflict saying %q", recorder.Code, failure, wantDetail)
			}
		})
	}
}

func TestAWriteWhoseListTimesOutSaysWhyTheListIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()
	fake.Fail["pending"] = fmt.Errorf("%s: %w after 10s", webtest.DataPath, proc.ErrTimedOut)

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()), http.MethodPost,
		taskPath(webtest.PlainUUID, "start"), "")

	// Assert
	list := webtest.Decode[api.TaskList](t, recorder)
	if fake.Count("start") != 1 || recorder.Code != http.StatusOK ||
		!strings.HasSuffix(list.Reason, "Taskwarrior did not answer in time") {
		t.Errorf("answer = %d %+v, calls = %q, want the start made, answered 200 with the read's reason",
			recorder.Code, list, fake.Asked())
	}
}

func TestAnAnswerThatIsNotTaskwarriorsJSONIsHandedToUnexpected(t *testing.T) {
	t.Parallel()

	// Arrange
	var heard []error

	fake := webtest.FakeTaskwarrior()
	fake.Fail["start"] = fmt.Errorf("%w: reading the export", taskwarrior.ErrBadOutput)
	deps := tasksDeps(fake)
	deps.Unexpected = func(_ config.Config, err error) { heard = append(heard, err) }

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, taskPath(webtest.StartedUUID,
		"start"), "")

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
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
	fake := webtest.FakeTaskwarrior()

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()), http.MethodPost,
		taskPath("not-a-uuid", "start"), "")

	// Assert
	if recorder.Code != http.StatusBadRequest || len(fake.Asked()) != 0 {
		t.Errorf("answer = %d, calls = %q, want 400 and Taskwarrior never asked", recorder.Code, fake.Asked())
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
			fake := webtest.FakeTaskwarrior()

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()),
				http.MethodPost, taskPath(webtest.StartedUUID, tt.write), tt.body)

			// Assert
			if recorder.Code != http.StatusUnprocessableEntity || len(fake.Asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 422 before Taskwarrior is asked", recorder.Code, fake.Asked())
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
			fake := webtest.FakeTaskwarrior()
			fake.Fail["pending"] = fmt.Errorf("reading %s: %w", webtest.DataPath, webtest.ErrSeam)

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()), http.MethodPost, write.path,
				write.body)

			// Assert
			list := webtest.Decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || list.Available || !strings.Contains(list.Reason, "could not be read again") {
				t.Errorf("answer = %d %+v, want 200 with the list marked unavailable, saying it could not be read again",
					recorder.Code, list)
			}

			if strings.Contains(recorder.Body.String(), webtest.DataPath) {
				t.Errorf("body = %q, names the data directory", recorder.Body.String())
			}
		})
	}
}

func TestAnAddThatLandedNamesItsTaskWhenTheListCannotBeReadAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()
	fake.Fail["pending"] = webtest.ErrSeam

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()), http.MethodPost, webtest.TasksPath,
		addBody)

	// Assert
	list := webtest.Decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || list.Added == nil || *list.Added != webtest.AddedUUID {
		t.Errorf("answer = %d, added = %v; want 200 naming the added task %s", recorder.Code, list.Added, webtest.AddedUUID)
	}
}

func TestATaskWriteWithoutItsSeamIs422(t *testing.T) {
	t.Parallel()

	for name, write := range taskWrites() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Tasks = seams.Tasks{}

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()), http.MethodPost, write.path, write.body)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
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
			fake := webtest.FakeTaskwarrior()
			dryRun := webserver.Info{Version: webtest.TestVersion, DryRun: true}
			handler := webtest.ServeWith(t, tasksDeps(fake), config.Default(), dryRun)

			// Act
			recorder := webtest.Send(t, handler, http.MethodPost, write.path, write.body)

			// Assert
			if recorder.Code != http.StatusForbidden || len(fake.Asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 403 and nothing asked of Taskwarrior", recorder.Code, fake.Asked())
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
			fake := webtest.FakeTaskwarrior()
			handler := webtest.Serve(t, tasksDeps(fake), config.Default())
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, write.path, strings.NewReader(write.body))
			request.Host = webtest.LoopbackHost
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://evil")

			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != http.StatusForbidden || len(fake.Asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want 403 and nothing asked of Taskwarrior", recorder.Code, fake.Asked())
			}
		})
	}
}
