// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
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
)

// The tasks the Taskwarrior fake holds, where it keeps them, and the path the
// list is read at.
const (
	tasksPath = "/api/tasks"
	// startedUUID is the started task's, linked to testKey; plainUUID an
	// unlinked task's; doneUUID a completed task's linked to another issue;
	// addedUUID the task an add creates.
	startedUUID = "5f3c9a1e-8b2d-4c6f-9e7a-1d2b3c4d5e6f"
	plainUUID   = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	doneUUID    = "9d8c7b6a-5f4e-4d3c-8b2a-1f0e9d8c7b6a"
	addedUUID   = "c0ffee00-1234-4abc-9def-0123456789ab"
	// oddUUID is the task whose status the spec does not know.
	oddUUID = "0dd0dd00-aaaa-4bbb-8ccc-ddddeeeeffff"
	// dataPath is where Taskwarrior keeps its data, and homePath your home,
	// which its errors can carry and no answer may repeat.
	dataPath = "/Users/x/.local/share/task"
	homePath = "/Users/x"
	// taskContext is the context the fake has active.
	taskContext = "work"
)

// taskFake is Taskwarrior as the server sees it: the install, the pending list
// and the linked tasks it answers, what undo and sync say, and each call made
// of it, in order. A call whose verb fail names answers that error instead. The
// stream calls it from the connection's goroutine, so it is locked.
type taskFake struct {
	mu      sync.Mutex
	install taskwarrior.Install
	pending taskwarrior.List
	linked  []taskwarrior.Task
	said    string
	fail    map[string]error
	calls   []string
}

// fakeTaskwarrior is a Taskwarrior with a sync backend and a context, whose
// pending list holds a started task for testKey and an unlinked one, and whose
// linked tasks are the started one and a completed one.
func fakeTaskwarrior() *taskFake {
	return &taskFake{
		install: taskwarrior.Install{
			Program: "/opt/homebrew/bin/task", Version: taskwarrior.MinimumVersion, DataDir: dataPath, SyncConfigured: true,
		},
		pending: taskwarrior.List{Tasks: []taskwarrior.Task{startedTask(), plainTask()}, Context: taskContext},
		linked:  []taskwarrior.Task{startedTask(), doneTask()},
		fail:    map[string]error{},
	}
}

// seams hands the fake to the server as its Taskwarrior.
func (f *taskFake) seams() seams.Tasks {
	return seams.Tasks{
		Install:  func() (taskwarrior.Install, error) { return f.install, f.called("install") },
		Pending:  func() (taskwarrior.List, error) { return f.pending, f.called("pending") },
		Linked:   func() ([]taskwarrior.Task, error) { return f.linked, f.called("linked") },
		Add:      func(line string) (string, error) { return addedUUID, f.called("add " + line) },
		Start:    func(uuid string) error { return f.called("start " + uuid) },
		Stop:     func(uuid string) error { return f.called("stop " + uuid) },
		Done:     func(uuid string) error { return f.called("done " + uuid) },
		Annotate: func(uuid, text string) error { return f.called("annotate " + uuid + " " + text) },
		Modify:   func(uuid, line string) error { return f.called("modify " + uuid + " " + line) },
		Undo:     func() (string, error) { return f.said, f.called("undo") },
		Sync:     func() (string, error) { return f.said, f.called("sync") },
	}
}

// called records call and answers the error fail names for its verb.
func (f *taskFake) called(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, call)
	verb, _, _ := strings.Cut(call, " ")

	return f.fail[verb]
}

// asked is every call made of the fake so far.
func (f *taskFake) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

// count is how many calls with verb were made of the fake.
func (f *taskFake) count(verb string) int {
	return len(slices.DeleteFunc(f.asked(), func(call string) bool {
		first, _, _ := strings.Cut(call, " ")

		return first != verb
	}))
}

// readAfter reports whether the pending list was read after call was made.
func (f *taskFake) readAfter(call string) bool {
	calls := f.asked()

	made := slices.Index(calls, call)

	return made >= 0 && slices.Contains(calls[made+1:], "pending")
}

// tasksDeps is filledDeps with the fake as its Taskwarrior, run from a home at
// homePath.
func tasksDeps(fake *taskFake) webserver.Deps {
	deps := filledDeps()
	deps.Tasks = fake.seams()
	deps.HomeDir = func() (string, error) { return homePath, nil }

	return deps
}

// taskDay is when the fake's tasks were made.
func taskDay() time.Time {
	return time.Date(2026, time.September, 21, 9, 30, 0, 0, time.UTC)
}

// startedTask is a pending task for testKey, started, with a due date and a note.
func startedTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: startedUUID, ID: 3, Description: testKey + ": " + testSummary, Status: taskwarrior.Pending,
		Project: "workflow", Priority: "H", Tags: []string{taskwarrior.LinkTag},
		Due: taskDay().Add(96 * time.Hour), Start: taskDay().Add(time.Hour),
		Entry: taskDay(), Modified: taskDay().Add(time.Hour), Urgency: 12.5,
		Annotations: []taskwarrior.Annotation{{Entry: taskDay(), Description: "https://jira.example/browse/" + testKey}},
		IssueKey:    testKey, IssueURL: "https://jira.example/browse/" + testKey,
	}
}

// plainTask is a pending task linked to no issue, not started, with no dates
// but the two every task has.
func plainTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: plainUUID, ID: 7, Description: "Water the plants", Status: taskwarrior.Pending,
		Entry: taskDay(), Modified: taskDay(), Urgency: 1.5,
	}
}

// doneTask is a completed task linked to another issue.
func doneTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: doneUUID, Description: "PROJ-7: Retire the old flag", Status: taskwarrior.Completed,
		Entry: taskDay(), Modified: taskDay(), End: taskDay().Add(2 * time.Hour), IssueKey: "PROJ-7",
	}
}

// oddTask is a started task linked to an issue whose status the spec does not
// know: one a synced replica holds, or whose status sanitizing emptied.
func oddTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: oddUUID, Description: "PROJ-8: From another replica", Status: "",
		Entry: taskDay(), Modified: taskDay(), Start: taskDay(), IssueKey: "PROJ-8",
	}
}

// withTasks is the fake Taskwarrior holding pending and linked instead.
func withTasks(pending, linked []taskwarrior.Task) seams.Tasks {
	fake := fakeTaskwarrior()
	fake.pending.Tasks, fake.linked = pending, linked

	return fake.seams()
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
	fake := fakeTaskwarrior()

	// Act
	recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if recorder.Code != http.StatusOK || !list.Available || list.Reason != "" || list.ReasonCode != nil {
		t.Fatalf("answer = %d %+v, want 200 with Taskwarrior available and no reason", recorder.Code, list)
	}

	if want := []string{startedUUID, plainUUID}; !slices.Equal(uuidsOf(list.Tasks), want) {
		t.Errorf("tasks = %v, want %v in the order Taskwarrior gave", uuidsOf(list.Tasks), want)
	}

	if list.Context != taskContext || !list.SyncAvailable || list.Said != "" || list.Added != nil {
		t.Errorf("list = %+v, want context %q, sync available, nothing said and nothing added", list, taskContext)
	}
}

func TestListTasksSaysWhetherASyncHasSomewhereToGo(t *testing.T) {
	t.Parallel()

	for _, syncConfigured := range []bool{true, false} {
		t.Run(fmt.Sprintf("sync configured %t", syncConfigured), func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.install.SyncConfigured = syncConfigured

			// Act
			recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

			// Assert
			want := fmt.Sprintf(`"sync_available":%t`, syncConfigured)
			if list := decode[api.TaskList](t, recorder); !list.Available || !strings.Contains(recorder.Body.String(), want) {
				t.Errorf("body = %s, want the list with %s", recorder.Body.String(), want)
			}
		})
	}
}

func TestListTasksAnswersNoPendingTaskAsAnEmptyList(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.pending.Tasks = nil

	// Act
	recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

	// Assert
	if body := recorder.Body.String(); recorder.Code != http.StatusOK || !strings.Contains(body, `"tasks":[]`) {
		t.Errorf("answer = %d %s, want 200 with tasks [], never null", recorder.Code, body)
	}
}

func TestListTasksLeavesOutATaskWhoseStatusTheSpecDoesNotKnow(t *testing.T) {
	t.Parallel()

	// Arrange
	fake := fakeTaskwarrior()
	fake.pending.Tasks = append(fake.pending.Tasks, oddTask())

	// Act
	recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

	// Assert
	list := decode[api.TaskList](t, recorder)
	if want := []string{startedUUID, plainUUID}; !slices.Equal(uuidsOf(list.Tasks), want) {
		t.Errorf("tasks = %v, want %v, never the task whose status the spec does not know", uuidsOf(list.Tasks), want)
	}
}

func TestListTasksSaysWhenTaskwarriorIsNotAvailable(t *testing.T) {
	t.Parallel()

	failing := func(err error) seams.Tasks {
		fake := fakeTaskwarrior()
		fake.fail["install"] = err

		return fake.seams()
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
		"no task program": {tasks: seams.Tasks{}, wantReason: notInstalled, wantCode: api.NotInstalled},
		"turned off": {
			tasks: seams.Tasks{}, disabled: true, wantReason: "Turned off by taskwarrior.disabled.", wantCode: api.TurnedOff,
		},
		"not installed": {
			tasks:      failing(fmt.Errorf("%w: tried %s", taskwarrior.ErrNotInstalled, program)),
			wantReason: notInstalled, wantCode: api.NotInstalled,
		},
		"not Taskwarrior": {
			tasks:      failing(fmt.Errorf("%w: tried %s", taskwarrior.ErrNotTaskwarrior, program)),
			wantReason: "The task on PATH is another program (go-task, most likely), not Taskwarrior.",
			wantCode:   api.NotTaskwarrior,
		},
		"too old": {
			tasks:      failing(fmt.Errorf("%w: 3.4.1 at %s; 3.5.0 or newer is needed", taskwarrior.ErrTooOld, program)),
			wantReason: "Taskwarrior is too old: 3.5.0 or newer is needed; workflow doctor shows the version found.",
			wantCode:   api.TooOld,
		},
		"never run": {
			tasks: failing(taskwarrior.NeverRunError{Program: program}),
			wantReason: "Taskwarrior has never been run: run it once in a terminal so it creates its " +
				"configuration; workflow doctor names the program.",
			wantCode: api.NeverRun,
		},
		"a malformed taskrc": {
			tasks:      failing(fmt.Errorf("%w: taskrc has a malformed line", taskwarrior.ErrRefused)),
			wantReason: "Taskwarrior's taskrc has a malformed line.", wantCode: api.MalformedTaskrc,
		},
		"a Taskwarrior that could not start": {
			tasks:      failing(fmt.Errorf("%w: %s", taskwarrior.ErrRefused, cannotInclude)),
			wantReason: "Taskwarrior could not start; workflow doctor says why.", wantCode: api.Unavailable,
		},
		"a search that could not finish": {
			tasks:      failing(fmt.Errorf("%w: looking for taskwarrior at %s", context.DeadlineExceeded, program)),
			wantReason: couldNotAsk, wantCode: api.Unavailable,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Tasks = tt.tasks
			cfg := config.Default()
			cfg.Taskwarrior.Disabled = tt.disabled

			// Act
			recorder := get(t, serve(t, deps, cfg), tasksPath)

			// Assert
			list := decode[api.TaskList](t, recorder)
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
	fake := fakeTaskwarrior()
	fake.fail["pending"] = fmt.Errorf("%s: %w after 10s", dataPath, proc.ErrTimedOut)

	// Act
	recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusBadGateway || failure.Detail != "Taskwarrior did not answer in time" {
		t.Errorf("answer = %d %+v, want 502 saying Taskwarrior did not answer in time", recorder.Code, failure)
	}
}

func TestAGitTimeoutIsNotWordedAsTaskwarriors(t *testing.T) {
	t.Parallel()

	// Arrange
	// Git's reads time out with the same sentinel Taskwarrior's do; only a
	// Taskwarrior read may say Taskwarrior did not answer.
	deps := filledDeps()
	deps.Branch = func() (gitrepo.Branch, error) {
		return gitrepo.Branch{}, fmt.Errorf("reading the branch: %w after 10s", proc.ErrTimedOut)
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/branch")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusInternalServerError || failure.Code != api.Internal {
		t.Errorf("answer = %d %+v, want the 500 internal problem a git read that timed out answers", recorder.Code, failure)
	}

	if strings.Contains(recorder.Body.String(), "Taskwarrior") {
		t.Errorf("a git timeout answered %s, which blames Taskwarrior", recorder.Body.String())
	}
}

func TestTheSnapshotCarriesTheTasksSummary(t *testing.T) {
	t.Parallel()

	startedPlain := plainTask()
	startedPlain.Start = taskDay()
	cases := map[string]struct {
		tasks         seams.Tasks
		wantAvailable bool
		wantActive    string
		wantLinked    int
	}{
		"a Taskwarrior that answers": {
			tasks: fakeTaskwarrior().seams(), wantAvailable: true, wantActive: startedUUID, wantLinked: 2,
		},
		"a started task only the pending list holds": {
			tasks:         withTasks([]taskwarrior.Task{startedPlain}, []taskwarrior.Task{doneTask()}),
			wantAvailable: true, wantActive: plainUUID, wantLinked: 1,
		},
		"a started task outside the context, which only the linked list holds": {
			tasks:         withTasks([]taskwarrior.Task{plainTask()}, []taskwarrior.Task{startedTask(), doneTask()}),
			wantAvailable: true, wantActive: startedUUID, wantLinked: 2,
		},
		"a started linked task whose status the spec does not know": {
			tasks:         withTasks([]taskwarrior.Task{plainTask()}, []taskwarrior.Task{oddTask(), doneTask()}),
			wantAvailable: true, wantActive: "", wantLinked: 1,
		},
		"an answer with no linked task the spec knows": {
			tasks:         withTasks([]taskwarrior.Task{plainTask()}, []taskwarrior.Task{oddTask()}),
			wantAvailable: true, wantActive: "", wantLinked: 0,
		},
		"no Taskwarrior": {tasks: seams.Tasks{}, wantAvailable: false, wantActive: "", wantLinked: 0},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := filledDeps()
			deps.Tasks = tt.tasks

			// Act
			body := streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String()

			// Assert
			summary := firstSnapshot(t, body).Tasks
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
			fake := fakeTaskwarrior()
			fake.fail[read] = errSeam

			// Act
			body := streamOnce(t, serve(t, tasksDeps(fake), config.Default()), "/api/events").Body.String()

			// Assert
			tasks := firstSnapshot(t, body).Tasks
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
		"frames a minute apart":    {between: time.Minute, wantAsked: streamedFrames},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := fakeTaskwarrior()
			fake.fail["install"] = taskwarrior.ErrNotTaskwarrior

			// Act
			pushed := snapshots(t, streamPaced(t, tasksDeps(fake), config.Default(), tt.between))

			// Assert
			if asked := fake.count("install"); asked != tt.wantAsked {
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
	fake := fakeTaskwarrior()

	// Act
	recorder := get(t, serve(t, tasksDeps(fake), config.Default()), tasksPath)

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
	if err != nil || !parsed.Equal(startedTask().Due) {
		t.Errorf("due = %q (%v), want %s in RFC 3339", due, err, startedTask().Due)
	}

	if absent, found := list.Tasks[1]["due"]; found {
		t.Errorf("a task with no due date carries due %s, want it absent", absent)
	}
}
