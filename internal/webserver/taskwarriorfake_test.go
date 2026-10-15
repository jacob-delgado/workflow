// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
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
		Install:  func(context.Context) (taskwarrior.Install, error) { return f.install, f.called("install") },
		Pending:  func(context.Context) (taskwarrior.List, error) { return f.pending, f.called("pending") },
		Linked:   func(context.Context) ([]taskwarrior.Task, error) { return f.linked, f.called("linked") },
		Add:      func(_ context.Context, line string) (string, error) { return addedUUID, f.called("add " + line) },
		Start:    func(_ context.Context, uuid string) error { return f.called("start " + uuid) },
		Stop:     func(_ context.Context, uuid string) error { return f.called("stop " + uuid) },
		Done:     func(_ context.Context, uuid string) error { return f.called("done " + uuid) },
		Annotate: func(_ context.Context, uuid, text string) error { return f.called("annotate " + uuid + " " + text) },
		Modify:   func(_ context.Context, uuid, line string) error { return f.called("modify " + uuid + " " + line) },
		Undo:     func(context.Context) (string, error) { return f.said, f.called("undo") },
		Sync:     func(context.Context) (string, error) { return f.said, f.called("sync") },
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
