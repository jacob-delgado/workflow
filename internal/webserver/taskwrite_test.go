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

func TestAWriteTaskwarriorRefusesIs422WithItsWords(t *testing.T) {
	t.Parallel()

	// Taskwarrior's words say what in the line it could not take; they keep
	// all but where its data is and which sync server a hook named.
	notADate := "'xyz' is not a valid date in the 'Y-M-D' format."
	refused := "Taskwarrior refused the command"
	cases := map[string]struct {
		words, wantDetail string
		// homeUnknown has no home directory known.
		homeUnknown bool
	}{
		"its words": {words: notADate, wantDetail: refused + ": " + notADate},
		"its words, with no home known": {
			words: notADate, homeUnknown: true, wantDetail: refused + ": " + notADate,
		},
		"its data directory": {
			words:      "Could not open " + dataPath + "/taskchampion.sqlite3.",
			wantDetail: refused + ": Could not open the data directory/taskchampion.sqlite3.",
		},
		"your home": {
			words:      "Hook " + homePath + "/.task/hooks/on-modify.check failed.",
			wantDetail: refused + ": Hook ~/.task/hooks/on-modify.check failed.",
		},
		"a line naming a URL": {
			words: "could not sync with https://" + syncServer + "/v1\n" + notADate, wantDetail: refused + ": " + notADate,
		},
		"a line naming a host and port": {
			words: notADate + "\nconnecting to " + syncServer + ":8080 failed", wantDetail: refused + ": " + notADate,
		},
		"a line naming an IPv4 address and port": {
			words: notADate + "\nretrying 192.168.1.10:10222", wantDetail: refused + ": " + notADate,
		},
		"a line naming an IPv6 address and port": {
			words: notADate + "\nretrying [fd00::17]:10222", wantDetail: refused + ": " + notADate,
		},
		"words on both sides of a line naming an address": {
			words:      "Hook on-modify.check failed.\nconnecting to " + syncServer + ":8080 failed\n" + notADate,
			wantDetail: refused + ": Hook on-modify.check failed.\n" + notADate,
		},
		"a clock time": {
			words: "The due date is 10:30 today.", wantDetail: refused + ": The due date is 10:30 today.",
		},
		"a date and time": {
			words:      "'2026-02-30T08:00' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": '2026-02-30T08:00' is not a valid date in the 'Y-M-D' format.",
		},
		"a date and time in a month past twelve": {
			words:      "'2026-13-02T10:30' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": '2026-13-02T10:30' is not a valid date in the 'Y-M-D' format.",
		},
		"a named date and time": {
			words:      "'tomorrowT10:00' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": 'tomorrowT10:00' is not a valid date in the 'Y-M-D' format.",
		},
		"a line naming a host and port beside a date and time": {
			words:      notADate + "\nat 2026-02-30T08:00 connecting to " + syncServer + ":8080 failed",
			wantDetail: refused + ": " + notADate,
		},
		// Run into a host's name, a date and time is read as part of the host
		// and its port, not taken out as a time before the address is looked for.
		"a line naming a host run into a date and time": {
			words:      notADate + "\nretrying " + syncServer + "2026-02-30T08:00",
			wantDetail: refused + ": " + notADate,
		},
		"a taskrc line it could not read": {
			words:      "Malformed entry 'sync.encryption_secret hunter2' in config file.\n" + notADate,
			wantDetail: refused + ": " + notADate,
		},
		"nothing but addresses": {words: "https://" + syncServer + "\n" + syncServer + ":8080", wantDetail: refused},
		"no words at all":       {words: "", wantDetail: refused},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail["modify"] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, tt.words)

			deps := tasksDeps(fake)
			if tt.homeUnknown {
				deps.Repositories.Home = ""
			}

			// Act
			recorder := send(t, serve(t, deps, config.Default()),
				http.MethodPost, taskPath(startedUUID, modifyWrite), `{"line":"due:xyz"}`)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != tt.wantDetail {
				t.Errorf("answer = %d %+v, want 422 with detail %q", recorder.Code, failure, tt.wantDetail)
			}
		})
	}
}

func TestARefusalKeepsWordsThatOnlyLookLikeAPlace(t *testing.T) {
	t.Parallel()

	// A data directory that is not an absolute path, and a home at the root,
	// name no place a word could be mistaken for: replacing them would rewrite
	// every "task" and every path.
	cases := map[string]struct {
		dataDir, home, words string
	}{
		"a relative data.location": {dataDir: "task", home: homePath, words: "Cannot modify a deleted task."},
		"a home at the root":       {dataDir: dataPath, home: "/", words: "Could not read /etc/taskrc."},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.install.DataDir = tt.dataDir
			fake.fail["modify"] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, tt.words)

			deps := tasksDeps(fake)
			deps.Repositories.Home = tt.home

			// Act
			recorder := send(t, serve(t, deps, config.Default()),
				http.MethodPost, taskPath(startedUUID, modifyWrite), `{"line":"due:xyz"}`)

			// Assert
			want := "Taskwarrior refused the command: " + tt.words
			if failure := decode[api.Problem](t, recorder); failure.Detail != want {
				t.Errorf("detail = %q, want %q", failure.Detail, want)
			}
		})
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
