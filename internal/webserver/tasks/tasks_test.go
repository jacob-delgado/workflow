// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tasks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	apispec "github.com/jacob-delgado/workflow/api"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// tasksDeps is webtest.FilledDeps with the fake as its Taskwarrior, run from a
// home at webtest.HomePath.
func tasksDeps(fake *webtest.TaskFake) webserver.Deps {
	deps := webtest.FilledDeps()
	deps.Tasks = fake.Seams()
	deps.Repositories.Home = webtest.HomePath

	return deps
}

// oddTask is a started task linked to an issue whose status the spec does not
// know: one a synced replica holds, or whose status sanitizing emptied.
func oddTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: webtest.OddUUID, Description: "PROJ-8: From another replica", Status: "",
		Entry: webtest.TaskDay(), Modified: webtest.TaskDay(), Start: webtest.TaskDay(), IssueKey: "PROJ-8",
	}
}

// withTasks is the fake Taskwarrior holding pending and linked instead.
func withTasks(pending, linked []taskwarrior.Task) seams.Tasks {
	fake := webtest.FakeTaskwarrior()
	fake.Pending.Tasks, fake.Linked = pending, linked

	return fake.Seams()
}

// uuidsOf is each task's uuid, in order.
func uuidsOf(tasks []api.Task) []string {
	uuids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		uuids = append(uuids, task.UUID)
	}

	return uuids
}

func TestListTasksAnswersThePendingList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

	// Assert
	list := webtest.Decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || !list.Available || list.Reason != "" || list.ReasonCode != nil {
		t.Fatalf("answer = %d %+v, want 200 with Taskwarrior available and no reason", recorder.Code, list)
	}

	if want := []string{webtest.StartedUUID, webtest.PlainUUID}; !slices.Equal(uuidsOf(list.Tasks), want) {
		t.Errorf("tasks = %v, want %v in the order Taskwarrior gave", uuidsOf(list.Tasks), want)
	}

	if list.Context != webtest.TaskContext || !list.SyncAvailable || list.Said != "" || list.Added != nil {
		t.Errorf("list = %+v, want context %q, sync available, nothing said and nothing added", list, webtest.TaskContext)
	}
}

func TestListTasksSaysWhetherASyncHasSomewhereToGo(t *testing.T) {
	t.Parallel()

	for _, syncConfigured := range []bool{true, false} {
		t.Run(fmt.Sprintf("sync configured %t", syncConfigured), func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			fake.Install.SyncConfigured = syncConfigured

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

			// Assert
			want := fmt.Sprintf(`"sync_available":%t`, syncConfigured)
			if list := webtest.Decode[api.TaskList](t, recorder); !list.Available || !strings.Contains(recorder.Body.String(),
				want) {
				t.Errorf("body = %s, want the list with %s", recorder.Body.String(), want)
			}
		})
	}
}

func TestListTasksAnswersNoPendingTaskAsAnEmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()
	fake.Pending.Tasks = nil

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

	// Assert
	if body := recorder.Body.String(); recorder.Code != http.StatusOK || !strings.Contains(body, `"tasks":[]`) {
		t.Errorf("answer = %d %s, want 200 with tasks [], never null", recorder.Code, body)
	}
}

func TestListTasksLeavesOutATaskWhoseStatusTheSpecDoesNotKnow(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()
	fake.Pending.Tasks = append(fake.Pending.Tasks, oddTask())

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

	// Assert
	list := webtest.Decode[api.TaskList](t, recorder)
	if want := []string{webtest.StartedUUID, webtest.PlainUUID}; !slices.Equal(uuidsOf(list.Tasks), want) {
		t.Errorf("tasks = %v, want %v, never the task whose status the spec does not know", uuidsOf(list.Tasks), want)
	}
}

func TestListTasksSaysWhenTaskwarriorIsNotAvailable(t *testing.T) {
	t.Parallel()

	failing := func(err error) seams.Tasks {
		fake := webtest.FakeTaskwarrior()
		fake.Fail["install"] = err

		return fake.Seams()
	}
	program := "/Users/x/bin/task"
	notInstalled := "Taskwarrior is not installed, or no task program is on PATH. " +
		"Install Taskwarrior 3.5.0 or newer, or set taskwarrior.program."
	couldNotAsk := "Taskwarrior could not be asked; workflow doctor says why."
	cannotInclude := "Could not read include file '" + program + "/dark.theme'."
	cases := map[string]struct {
		tasks      seams.Tasks
		disabled   bool
		wantReason string
		wantCode   api.TaskListReasonCode
	}{
		"no task program": {tasks: seams.Tasks{}, wantReason: notInstalled, wantCode: api.TaskListReasonCodeNotInstalled},
		"turned off": {
			tasks: seams.Tasks{}, disabled: true,
			wantReason: "Turned off by taskwarrior.disabled.", wantCode: api.TaskListReasonCodeTurnedOff,
		},
		"not installed": {
			tasks:      failing(fmt.Errorf("%w: tried %s", taskwarrior.ErrNotInstalled, program)),
			wantReason: notInstalled, wantCode: api.TaskListReasonCodeNotInstalled,
		},
		"not Taskwarrior": {
			tasks: failing(fmt.Errorf("%w: tried %s", taskwarrior.ErrNotTaskwarrior, program)),
			wantReason: "The task on PATH is another program (go-task, most likely), not Taskwarrior. " +
				"Set taskwarrior.program to Taskwarrior's path, then restart workflow; workflow doctor names what it found.",
			wantCode: api.TaskListReasonCodeNotTaskwarrior,
		},
		"too old": {
			tasks:      failing(fmt.Errorf("%w: 3.4.1 at %s; 3.5.0 or newer is needed", taskwarrior.ErrTooOld, program)),
			wantReason: "Taskwarrior is too old: 3.5.0 or newer is needed; workflow doctor shows the version found.",
			wantCode:   api.TaskListReasonCodeTooOld,
		},
		"never run": {
			tasks: failing(taskwarrior.NeverRunError{Program: program}),
			wantReason: "Taskwarrior has never been run: run it once in a terminal so it creates its " +
				"configuration; workflow doctor names the program.",
			wantCode: api.TaskListReasonCodeNeverRun,
		},
		"a malformed taskrc": {
			tasks:      failing(fmt.Errorf("%w: taskrc has a malformed line", taskwarrior.ErrRefused)),
			wantReason: "Taskwarrior's taskrc has a malformed line.", wantCode: api.TaskListReasonCodeMalformedTaskrc,
		},
		"a Taskwarrior that could not start": {
			tasks:      failing(fmt.Errorf("%w: %s", taskwarrior.ErrRefused, cannotInclude)),
			wantReason: "Taskwarrior could not start; workflow doctor says why.", wantCode: api.TaskListReasonCodeUnavailable,
		},
		"a search that could not finish": {
			tasks:      failing(fmt.Errorf("%w: looking for taskwarrior at %s", context.DeadlineExceeded, program)),
			wantReason: couldNotAsk, wantCode: api.TaskListReasonCodeUnavailable,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Tasks = tt.tasks
			cfg := config.Default()
			cfg.Taskwarrior.Disabled = tt.disabled

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, deps, cfg), webtest.TasksPath)

			// Assert
			list := webtest.Decode[api.TaskList](t, recorder)
			if recorder.Code != http.StatusOK || list.Available || list.Reason != tt.wantReason {
				t.Errorf("answer = %d %+v, want 200, not available, reason %q", recorder.Code, list, tt.wantReason)
			}

			if list.ReasonCode == nil || *list.ReasonCode != tt.wantCode {
				t.Errorf("reason_code = %v, want %q", list.ReasonCode, tt.wantCode)
			}

			if body := recorder.Body.String(); !strings.Contains(body, `"tasks":[]`) || strings.Contains(body, program) {
				t.Errorf("body = %s, want tasks [] and never the program's path", body)
			}
		})
	}
}

func TestListTasksAnswersAFailedReadAsAProblem(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()
	fake.Fail["pending"] = fmt.Errorf("%s: %w after 10s", webtest.DataPath, proc.ErrTimedOut)

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Detail != "Taskwarrior did not answer in time" {
		t.Errorf("answer = %d %+v, want 502 saying Taskwarrior did not answer in time", recorder.Code, failure)
	}
}

func TestAGitTimeoutIsNotWordedAsTaskwarriors(t *testing.T) {
	t.Parallel()

	// Arrange
	// Git's reads time out with the same sentinel Taskwarrior's do; only a
	// Taskwarrior read may say Taskwarrior did not answer.
	deps := webtest.FilledDeps()
	deps.Git.Branch = func(context.Context) (gitrepo.Branch, error) {
		return gitrepo.Branch{}, fmt.Errorf("reading the branch: %w after 10s", proc.ErrTimedOut)
	}

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/branch")

	// Assert
	failure := webtest.Decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusInternalServerError || failure.Code != api.ProblemCodeInternal {
		t.Errorf("answer = %d %+v, want the 500 internal problem a git read that timed out answers", recorder.Code, failure)
	}

	if strings.Contains(recorder.Body.String(), "Taskwarrior") {
		t.Errorf("a git timeout answered %s, which blames Taskwarrior", recorder.Body.String())
	}
}

func TestTheSnapshotCarriesTheTasksSummary(t *testing.T) {
	t.Parallel()

	startedPlain := webtest.PlainTask()
	startedPlain.Start = webtest.TaskDay()
	cases := map[string]struct {
		tasks         seams.Tasks
		wantAvailable bool
		wantActive    string
		wantLinked    int
	}{
		"a Taskwarrior that answers": {
			tasks: webtest.FakeTaskwarrior().Seams(), wantAvailable: true, wantActive: webtest.StartedUUID, wantLinked: 2,
		},
		"a started task only the pending list holds": {
			tasks:         withTasks([]taskwarrior.Task{startedPlain}, []taskwarrior.Task{webtest.DoneTask()}),
			wantAvailable: true, wantActive: webtest.PlainUUID, wantLinked: 1,
		},
		"a started task outside the context, which only the linked list holds": {
			tasks: withTasks([]taskwarrior.Task{webtest.PlainTask()}, []taskwarrior.Task{
				webtest.StartedTask(),
				webtest.DoneTask(),
			}),
			wantAvailable: true, wantActive: webtest.StartedUUID, wantLinked: 2,
		},
		"a started linked task whose status the spec does not know": {
			tasks:         withTasks([]taskwarrior.Task{webtest.PlainTask()}, []taskwarrior.Task{oddTask(), webtest.DoneTask()}),
			wantAvailable: true, wantActive: "", wantLinked: 1,
		},
		"an answer with no linked task the spec knows": {
			tasks:         withTasks([]taskwarrior.Task{webtest.PlainTask()}, []taskwarrior.Task{oddTask()}),
			wantAvailable: true, wantActive: "", wantLinked: 0,
		},
		"no Taskwarrior": {tasks: seams.Tasks{}, wantAvailable: false, wantActive: "", wantLinked: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := webtest.FilledDeps()
			deps.Tasks = tt.tasks

			// Act
			body := webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()), "/api/events").Body.String()

			// Assert
			summary := webtest.FirstSnapshot(t, body).Tasks
			if summary.Available != tt.wantAvailable || activeUUID(summary) != tt.wantActive ||
				len(summary.Linked) != tt.wantLinked {
				t.Errorf("tasks = %+v, want available %t, active %q and %d linked",
					summary, tt.wantAvailable, tt.wantActive, tt.wantLinked)
			}

			if strings.Contains(body, `"linked":[]`) != (tt.wantLinked == 0) {
				t.Errorf("body = %s, want linked an array, never null", body)
			}

			checkSummaryKeepsToTheSpec(t, body)
		})
	}
}

// checkSummaryKeepsToTheSpec fails the test when the first frame's tasks break
// the spec's TasksSummary schema, which the page parses every frame by.
func checkSummaryKeepsToTheSpec(t *testing.T, body string) {
	t.Helper()

	_, payload, _ := strings.Cut(body, "data: ")
	payload, _, _ = strings.Cut(payload, "\n")

	var frame struct {
		Tasks any `json:"tasks"`
	}

	err := json.Unmarshal([]byte(payload), &frame)
	if err != nil {
		t.Fatalf("decoding the frame %q: %v", payload, err)
	}

	spec, err := openapi3.NewLoader().LoadFromData(apispec.Spec)
	if err != nil {
		t.Fatalf("loading the spec: %v", err)
	}

	err = spec.Components.Schemas["TasksSummary"].Value.VisitJSON(frame.Tasks)
	if err != nil {
		t.Errorf("the frame's tasks break the spec's TasksSummary: %v", err)
	}
}

// activeUUID is the summary's active task's uuid, or "" when none is active.
func activeUUID(summary api.TasksSummary) string {
	if summary.Active == nil {
		return ""
	}

	return summary.Active.UUID
}

func TestTheSnapshotIsUnavailableUntilTaskwarriorHasAnsweredBothReads(t *testing.T) {
	t.Parallel()

	for _, read := range []string{pendingRead, "linked"} {
		t.Run(read, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			fake.Fail[read] = webtest.ErrSeam

			// Act
			body := webtest.StreamOnce(t, webtest.Serve(t, tasksDeps(fake), config.Default()), "/api/events").Body.String()

			// Assert
			tasks := webtest.FirstSnapshot(t, body).Tasks
			if tasks.Available || tasks.Reason != "Taskwarrior could not be read; workflow doctor says why." ||
				tasks.Active != nil || !strings.Contains(body, `"linked":[]`) {
				t.Errorf("tasks = %+v in %s, want Taskwarrior not available, the fixed reason, no active task "+
					"and linked []", tasks, body)
			}

			checkSummaryKeepsToTheSpec(t, body)
		})
	}
}

func TestTheSnapshotHoldsAFailedSearchForTaskwarriorForAMinute(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		between   time.Duration
		wantAsked int
	}{
		"frames within the minute": {between: 0, wantAsked: 1},
		"frames a minute apart":    {between: time.Minute, wantAsked: webtest.StreamedFrames},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			fake.Fail["install"] = taskwarrior.ErrNotTaskwarrior

			// Act
			pushed := webtest.Snapshots(t, webtest.StreamPaced(t, tasksDeps(fake), config.Default(), tt.between))

			// Assert
			if asked := fake.Count("install"); asked != tt.wantAsked {
				t.Errorf("looked for Taskwarrior %d times over %d frames, want %d", asked, len(pushed), tt.wantAsked)
			}

			if last := pushed[len(pushed)-1].Tasks; last.Available || last.Reason == "" {
				t.Errorf("last frame's tasks = %+v, want not available, with the reason", last)
			}
		})
	}
}

func TestTaskDatesAreRFC3339(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := webtest.FakeTaskwarrior()

	// Act
	recorder := webtest.Get(t, webtest.Serve(t, tasksDeps(fake), config.Default()), webtest.TasksPath)

	// Assert
	var list struct {
		Tasks []map[string]json.RawMessage `json:"tasks"`
	}

	err := json.Unmarshal(recorder.Body.Bytes(), &list)
	if err != nil || len(list.Tasks) != 2 {
		t.Fatalf("decoding %s: %v, want two tasks", recorder.Body.String(), err)
	}

	var due string

	err = json.Unmarshal(list.Tasks[0]["due"], &due)
	if err != nil {
		t.Fatalf("the started task's due = %s: %v, want a string", list.Tasks[0]["due"], err)
	}

	parsed, err := time.Parse(time.RFC3339, due)
	if err != nil || !parsed.Equal(webtest.StartedTask().Due) {
		t.Errorf("due = %q (%v), want %s in RFC 3339", due, err, webtest.StartedTask().Due)
	}

	if absent, found := list.Tasks[1]["due"]; found {
		t.Errorf("a task with no due date carries due %s, want it absent", absent)
	}
}
