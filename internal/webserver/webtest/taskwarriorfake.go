// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

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
	TasksPath = "/api/tasks"
	// StartedUUID is the started task's, linked to TestKey; PlainUUID an
	// unlinked task's; DoneUUID a completed task's linked to another issue;
	// AddedUUID the task an add creates.
	StartedUUID = "5f3c9a1e-8b2d-4c6f-9e7a-1d2b3c4d5e6f"
	PlainUUID   = "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	DoneUUID    = "9d8c7b6a-5f4e-4d3c-8b2a-1f0e9d8c7b6a"
	AddedUUID   = "c0ffee00-1234-4abc-9def-0123456789ab"
	// OddUUID is the task whose status the spec does not know.
	OddUUID = "0dd0dd00-aaaa-4bbb-8ccc-ddddeeeeffff"
	// DataPath is where Taskwarrior keeps its data, and HomePath your home,
	// which its errors can carry and no answer may repeat.
	DataPath = "/Users/x/.local/share/task"
	HomePath = "/Users/x"
	// TaskContext is the context the fake has active.
	TaskContext = "work"
)

// TaskFake is Taskwarrior as the server sees it: the install, the pending list
// and the linked tasks it answers, what undo and sync say, and each call made
// of it, in order. A call whose verb Fail names answers that error instead. The
// stream calls it from the connection's goroutine, so it is locked.
type TaskFake struct {
	mu      sync.Mutex
	Install taskwarrior.Install
	Pending taskwarrior.List
	Linked  []taskwarrior.Task
	Said    string
	Fail    map[string]error
	calls   []string
}

// FakeTaskwarrior is a Taskwarrior with a sync backend and a context, whose
// pending list holds a started task for TestKey and an unlinked one, and whose
// linked tasks are the started one and a completed one.
func FakeTaskwarrior() *TaskFake {
	return &TaskFake{
		Install: taskwarrior.Install{
			Program: "/opt/homebrew/bin/task", Version: taskwarrior.MinimumVersion, DataDir: DataPath, SyncConfigured: true,
		},
		Pending: taskwarrior.List{Tasks: []taskwarrior.Task{StartedTask(), PlainTask()}, Context: TaskContext},
		Linked:  []taskwarrior.Task{StartedTask(), DoneTask()},
		Fail:    map[string]error{},
	}
}

// Seams hands the fake to the server as its Taskwarrior.
func (f *TaskFake) Seams() seams.Tasks {
	return seams.Tasks{
		Install:  func(context.Context) (taskwarrior.Install, error) { return f.Install, f.called("install") },
		Pending:  func(context.Context) (taskwarrior.List, error) { return f.Pending, f.called("pending") },
		Linked:   func(context.Context) ([]taskwarrior.Task, error) { return f.Linked, f.called("linked") },
		Add:      func(_ context.Context, line string) (string, error) { return AddedUUID, f.called("add " + line) },
		Start:    func(_ context.Context, uuid string) error { return f.called("start " + uuid) },
		Stop:     func(_ context.Context, uuid string) error { return f.called("stop " + uuid) },
		Done:     func(_ context.Context, uuid string) error { return f.called("done " + uuid) },
		Annotate: func(_ context.Context, uuid, text string) error { return f.called("annotate " + uuid + " " + text) },
		Modify:   func(_ context.Context, uuid, line string) error { return f.called("modify " + uuid + " " + line) },
		Undo:     func(context.Context) (string, error) { return f.Said, f.called("undo") },
		Sync:     func(context.Context) (string, error) { return f.Said, f.called("sync") },
	}
}

// Asked is every call made of the fake so far.
func (f *TaskFake) Asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.calls)
}

// Count is how many calls with verb were made of the fake.
func (f *TaskFake) Count(verb string) int {
	return len(slices.DeleteFunc(f.Asked(), func(call string) bool {
		first, _, _ := strings.Cut(call, " ")

		return first != verb
	}))
}

// ReadAfter reports whether the pending list was read after call was made.
func (f *TaskFake) ReadAfter(call string) bool {
	calls := f.Asked()

	made := slices.Index(calls, call)

	return made >= 0 && slices.Contains(calls[made+1:], "pending")
}

// called records call and answers the error Fail names for its verb.
func (f *TaskFake) called(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, call)
	verb, _, _ := strings.Cut(call, " ")

	return f.Fail[verb]
}

// TaskDay is when the fake's tasks were made.
func TaskDay() time.Time {
	return time.Date(2026, time.September, 21, 9, 30, 0, 0, time.UTC)
}

// StartedTask is a pending task for TestKey, started, with a due date and a note.
func StartedTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: StartedUUID, ID: 3, Description: TestKey + ": " + TestSummary, Status: taskwarrior.Pending,
		Project: "workflow", Priority: "H", Tags: []string{taskwarrior.LinkTag},
		Due: TaskDay().Add(96 * time.Hour), Start: TaskDay().Add(time.Hour),
		Entry: TaskDay(), Modified: TaskDay().Add(time.Hour), Urgency: 12.5,
		Annotations: []taskwarrior.Annotation{{Entry: TaskDay(), Description: "https://jira.example/browse/" + TestKey}},
		IssueKey:    TestKey, IssueURL: "https://jira.example/browse/" + TestKey,
	}
}

// PlainTask is a pending task linked to no issue, not started, with no dates
// but the two every task has.
func PlainTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: PlainUUID, ID: 7, Description: "Water the plants", Status: taskwarrior.Pending,
		Entry: TaskDay(), Modified: TaskDay(), Urgency: 1.5,
	}
}

// DoneTask is a completed task linked to another issue.
func DoneTask() taskwarrior.Task {
	return taskwarrior.Task{
		UUID: DoneUUID, Description: "PROJ-7: Retire the old flag", Status: taskwarrior.Completed,
		Entry: TaskDay(), Modified: TaskDay(), End: TaskDay().Add(2 * time.Hour), IssueKey: "PROJ-7",
	}
}
