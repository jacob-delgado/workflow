// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Where the writes that name no task are sent, and the issue's page a tracked
// task is annotated with.
const (
	trackPath = tasksPath + "/track"
	undoPath  = tasksPath + "/undo"
	syncPath  = tasksPath + "/sync"
	issuePage = "https://jira.example/browse/" + testKey
	// trackBody asks to track testKey.
	trackBody = `{"issue_key":"` + testKey + `"}`
	// addVerb is how the fake records an add, which tracking makes too, and
	// startVerb, stopVerb, doneVerb, undoVerb, annotateVerb and pendingRead how
	// it records a start, a stop, a done, an undo, an annotation and the read of
	// the pending list.
	addVerb      = "add"
	startVerb    = "start"
	stopVerb     = "stop"
	doneVerb     = "done"
	undoVerb     = "undo"
	annotateVerb = "annotate"
	pendingRead  = "pending"
	// addBody asks to add a task.
	addBody = `{"line":"Water the plants"}`
	// syncServer is the sync server a hook's feedback can name, which no answer
	// may repeat.
	syncServer = "sync.internal.example"
)

// leakyWords is Taskwarrior's words when they name what no answer may repeat:
// a file in its data directory, one in your home, and the sync server, by URL
// and by host and port.
func leakyWords() string {
	return strings.Join([]string{
		"Found existing '*.data' files in " + dataPath,
		"Hook " + homePath + "/.task/hooks/on-exit.sync said:",
		"could not sync with https://" + syncServer,
		"connecting to " + syncServer + ":8080 failed",
	}, "\n")
}

// trackableDeps is tasksDeps with the tracker's page for each issue.
func trackableDeps(fake *taskFake) webserver.Deps {
	deps := tasksDeps(fake)
	deps.Jira.BrowseURL = func(key jira.Key) string { return "https://jira.example/browse/" + string(key) }

	return deps
}

func TestAddTaskSendsTheLineAndAnswersTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()),
		http.MethodPost, tasksPath, `{"line":"Water the plants project:home"}`)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || !list.Available || !fake.readAfter("add Water the plants project:home") {
		t.Errorf("answer = %d %+v, calls = %q, want 200 with the list read after the add", recorder.Code, list, fake.asked())
	}

	if list.Added == nil || *list.Added != addedUUID {
		t.Errorf("added = %v, want the new task's uuid %s", list.Added, addedUUID)
	}
}

func TestAddTaskRequiresALine(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, tasksPath, `{"line":"  "}`)

	// Assert
	if recorder.Code != http.StatusUnprocessableEntity || len(fake.asked()) != 0 {
		t.Errorf("answer = %d, calls = %q, want 422 before Taskwarrior is asked", recorder.Code, fake.asked())
	}
}

func TestTrackIssueBuildsTheLineFromTheIssueAndAnnotatesIt(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	deps := trackableDeps(fake)
	deps.Jira.Issue = func(key jira.Key) (jira.IssueDetail, error) {
		return jira.IssueDetail{Issue: jira.Issue{Key: key, Summary: testSummary, Priority: "High"}}, nil
	}

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, trackPath, trackBody)

	// Assert
	add := "add jiraid:" + testKey + " jiraurl:" + issuePage + " +jira priority:H -- " + testKey + ": " + testSummary
	annotate := "annotate " + addedUUID + " " + issuePage

	calls := fake.asked()
	if added := slices.Index(calls, add); added < 0 || slices.Index(calls, annotate) < added {
		t.Fatalf("calls = %q, want %q and then %q", calls, add, annotate)
	}

	list := decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || list.Said != "" || !fake.readAfter(annotate) {
		t.Errorf("answer = %d %+v, want 200 with the list read after the annotation", recorder.Code, list)
	}

	if list.Added == nil || *list.Added != addedUUID {
		t.Errorf("added = %v, want the issue's new task's uuid %s", list.Added, addedUUID)
	}
}

func TestTrackIssueWithoutAPageAddsNoAnnotation(t *testing.T) {
	t.Parallel()

	// Arrange
	// The forge's own issues have no page to link, and Taskwarrior refuses an
	// empty annotation.
	fake := fakeTaskwarrior()

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, trackPath, trackBody)

	// Assert
	if recorder.Code != http.StatusOK || fake.count("add") != 1 || fake.count("annotate") != 0 {
		t.Errorf("answer = %d, calls = %q, want 200 with the task added and not annotated", recorder.Code, fake.asked())
	}

	if list := decode[api.TaskList](t, recorder); list.Added == nil || *list.Added != addedUUID {
		t.Errorf("added = %v, want the issue's new task's uuid %s", list.Added, addedUUID)
	}
}

func TestTrackIssueOfAnUnknownIssueIs404(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	deps := trackableDeps(fake)
	deps.Jira.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, jira.ErrNotFound }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, trackPath, trackBody)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.ProblemCodeNotFound || fake.count("add") != 0 {
		t.Errorf("answer = %d %+v, calls = %q, want 404 and no task added", recorder.Code, failure, fake.asked())
	}

	if want := "issue " + testKey + " was not found"; failure.Detail != want {
		t.Errorf("detail = %q, want %q, as GET /api/issues/{key} says it", failure.Detail, want)
	}
}

func TestTrackIssueIsRefusedWithoutWhatItNeeds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body       string
		mutate     func(*webserver.Deps)
		wantStatus int
	}{
		"no issue key": {
			body: `{"issue_key":""}`, mutate: func(*webserver.Deps) {}, wantStatus: http.StatusUnprocessableEntity,
		},
		"no tracker": {
			body: trackBody, wantStatus: http.StatusUnprocessableEntity,
			mutate: func(deps *webserver.Deps) { deps.Jira.Issue = nil },
		},
		"a forge that will not show the repository": {
			body: trackBody, wantStatus: http.StatusNotFound,
			mutate: func(deps *webserver.Deps) {
				deps.Jira.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, forge.ErrNoRepository }
			},
		},
		"a tracker that cannot be reached": {
			body: trackBody, wantStatus: http.StatusBadGateway,
			mutate: func(deps *webserver.Deps) {
				deps.Jira.Issue = func(jira.Key) (jira.IssueDetail, error) { return jira.IssueDetail{}, jira.ErrUnreachable }
			},
		},
		// Words after the key would reach Taskwarrior as words of their own,
		// ahead of the line's --.
		"a tracker key that is not one word": {
			body: trackBody, wantStatus: http.StatusUnprocessableEntity,
			mutate: func(deps *webserver.Deps) {
				deps.Jira.Issue = func(jira.Key) (jira.IssueDetail, error) {
					return jira.IssueDetail{Issue: jira.Issue{Key: testKey + " rc.hooks.location=/tmp", Summary: testSummary}}, nil
				}
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			deps := trackableDeps(fake)
			tt.mutate(&deps)

			// Act
			recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, trackPath, tt.body)

			// Assert
			if recorder.Code != tt.wantStatus || len(fake.asked()) != 0 {
				t.Errorf("answer = %d, calls = %q, want %d and nothing asked of Taskwarrior",
					recorder.Code, fake.asked(), tt.wantStatus)
			}
		})
	}
}

func TestTrackIssueWhoseAnnotationFailsStillAnswersTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.fail["annotate"] = fmt.Errorf("%s: %w after 30s", dataPath, proc.ErrTimedOut)

	// Act
	recorder := send(t, serve(t, trackableDeps(fake), config.Default()), http.MethodPost, trackPath, trackBody)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if want := "created but not annotated: Taskwarrior did not answer in time"; recorder.Code != http.StatusOK ||
		list.Said != want || len(list.Tasks) != 2 {
		t.Errorf("answer = %d %+v, want 200 with the list and said %q", recorder.Code, list, want)
	}

	if list.Added == nil || *list.Added != addedUUID {
		t.Errorf("added = %v, want the task created, %s", list.Added, addedUUID)
	}
}

func TestUndoAndSyncAnswerWhatTaskwarriorSaid(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		path       string
		said       string
		fail       error
		wantStatus int
		wantText   string
	}{
		"an undo": {
			path: undoPath, said: "reverted 2 operations", wantStatus: http.StatusOK, wantText: `"said":"reverted 2 operations"`,
		},
		"a sync": {
			path: syncPath, said: "Sync successful.", wantStatus: http.StatusOK, wantText: `"said":"Sync successful."`,
		},
		"a sync with nowhere to go": {
			path: syncPath, fail: taskwarrior.ErrNoSync, wantStatus: http.StatusUnprocessableEntity,
			wantText: "No sync backend is set in your taskrc, so there is nowhere to sync. " +
				"Set one of the sync.* settings (task-sync(5)).",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.said = tt.said
			verb := strings.TrimPrefix(tt.path, tasksPath+"/")
			fake.fail[verb] = tt.fail

			// Act
			recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, tt.path, "")

			// Assert
			body := recorder.Body.String()
			if recorder.Code != tt.wantStatus || !strings.Contains(body, tt.wantText) || strings.Contains(body, `"added"`) {
				t.Errorf("answer = %d %s, want %d carrying %s and nothing added", recorder.Code, body, tt.wantStatus, tt.wantText)
			}
		})
	}
}

func TestTaskCommandsAnswerARefusalInTaskwarriorsWords(t *testing.T) {
	t.Parallel()

	words := "Hook Error: Expected feedback from failing hook script: on-add.check"
	cases := map[string]struct {
		verb, path, body string
	}{
		"an add":  {verb: addVerb, path: tasksPath, body: addBody},
		"a track": {verb: addVerb, path: trackPath, body: trackBody},
		"an undo": {verb: undoVerb, path: undoPath},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail[tt.verb] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, words)

			// Act
			recorder := send(t, serve(t, trackableDeps(fake), config.Default()), http.MethodPost, tt.path, tt.body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != "Taskwarrior refused the command: "+words {
				t.Errorf("answer = %d %+v, want 422 with Taskwarrior's words", recorder.Code, failure)
			}
		})
	}
}

func TestUndoWithNothingToUndoIs409(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.fail["undo"] = taskwarrior.ErrNothingChanged

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, undoPath, "")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Detail != "Taskwarrior has nothing to undo" {
		t.Errorf("answer = %d %+v, want 409 saying Taskwarrior has nothing to undo", recorder.Code, failure)
	}
}

func TestARefusedSyncNeverRepeatsTaskwarriorsWords(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refused sync names the sync server, an internal host.
	host := "sync.corp.internal"
	fake := fakeTaskwarrior()
	fake.fail["sync"] = fmt.Errorf("%w: could not reach https://%s:8080 for %s", taskwarrior.ErrRefused, host, dataPath)

	// Act
	recorder := send(t, serve(t, tasksDeps(fake), config.Default()), http.MethodPost, syncPath, "")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity ||
		failure.Detail != "Taskwarrior could not sync; run task sync in a terminal to see why" {
		t.Errorf("answer = %d %+v, want 422 in fixed words", recorder.Code, failure)
	}

	if body := recorder.Body.String(); strings.Contains(body, host) || strings.Contains(body, dataPath) {
		t.Errorf("body = %s, which repeats Taskwarrior's words", body)
	}
}

func TestTaskFaultsNeverLeakAPath(t *testing.T) {
	t.Parallel()

	start := taskPath(startedUUID, "start")
	cases := map[string]struct {
		method, path, body string
		// verb is the call that fails, with cause wrapped around leakyWords.
		verb       string
		cause      error
		wantStatus int
		wantCode   api.ProblemCode
	}{
		"nothing changed": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrNothingChanged,
			wantStatus: http.StatusConflict, wantCode: api.ProblemCodeConflict,
		},
		"not installed": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrNotInstalled,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeNotSetUp,
		},
		"not Taskwarrior": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrNotTaskwarrior,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeNotSetUp,
		},
		"too old": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrTooOld,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		"never run": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrNotConfigured,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeNotSetUp,
		},
		"no sync backend": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrNoSync,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		"an unreadable answer": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrBadOutput,
			wantStatus: http.StatusInternalServerError, wantCode: api.ProblemCodeInternal,
		},
		"timed out": {
			method: http.MethodPost, path: start, verb: startVerb, cause: proc.ErrTimedOut,
			wantStatus: http.StatusBadGateway, wantCode: api.ProblemCodeUnreachable,
		},
		"a refused start": {
			method: http.MethodPost, path: start, verb: startVerb, cause: taskwarrior.ErrRefused,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		"a refused add": {
			method: http.MethodPost, path: tasksPath, body: addBody, verb: addVerb,
			cause: taskwarrior.ErrRefused, wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		"a refused undo": {
			method: http.MethodPost, path: undoPath, verb: undoVerb, cause: taskwarrior.ErrRefused,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		"a refused read of the list": {
			method: http.MethodGet, path: tasksPath, verb: pendingRead, cause: taskwarrior.ErrRefused,
			wantStatus: http.StatusUnprocessableEntity, wantCode: api.ProblemCodeUnprocessable,
		},
		// The start is made, so it answers the list, unavailable with why.
		"a refused read of the list after a write": {
			method: http.MethodPost, path: start, verb: pendingRead, cause: taskwarrior.ErrRefused,
			wantStatus: http.StatusOK,
		},
		// The task is created, so the track answers the list, and said carries
		// the refusal: the web's form of the terminal's ErrAnnotateFailed.
		"a track whose annotation is refused": {
			method: http.MethodPost, path: trackPath, body: trackBody, verb: annotateVerb, cause: taskwarrior.ErrRefused,
			wantStatus: http.StatusOK,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail[tt.verb] = fmt.Errorf("%w: %s", tt.cause, leakyWords())

			// Act
			recorder := send(t, serve(t, trackableDeps(fake), config.Default()), tt.method, tt.path, tt.body)

			// Assert
			answer := decode[struct {
				Code   api.ProblemCode `json:"code"`
				Detail string          `json:"detail"`
				Said   string          `json:"said"`
				Reason string          `json:"reason"`
			}](t, recorder)
			if fake.count(tt.verb) != 1 || recorder.Code != tt.wantStatus || answer.Code != tt.wantCode ||
				answer.Detail+answer.Said+answer.Reason == "" {
				t.Errorf("answer = %d %s, calls = %q, want %d %q saying why", recorder.Code, recorder.Body.String(),
					fake.asked(), tt.wantStatus, tt.wantCode)
			}

			for _, leak := range []string{dataPath, homePath, syncServer} {
				if strings.Contains(recorder.Body.String(), leak) {
					t.Errorf("body = %s, which names %s", recorder.Body.String(), leak)
				}
			}
		})
	}
}
