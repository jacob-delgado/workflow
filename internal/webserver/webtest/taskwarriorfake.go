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
// of it, in order. A call whose verb Fail names answers that error instead,
// and a refused write changes nothing. A write whose effect needs none of
// Taskwarrior's grammar changes the tasks as Taskwarrior would, at writtenAt:
// an add appends the pending task AddedUUID, its description the line as
// given, and each add answers that one UUID, so a test adds once per fake; a
// start starts the task it names and a stop stops it; a done completes it,
// out of the pending list and, where it is linked, still in the linked
// tasks; an annotation adds its note. A modify, an undo and a sync
// leave the tasks as they are, since their effect is Taskwarrior's own grammar
// or history, which the fake does not reimplement; and the fake declines no
// write on its own, only those Fail names. The stream calls it from the
// connection's goroutine, so it is locked, and a call is recorded under the
// same lock as the read or change it makes: a read recorded after a write
// sees it.
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
		Install: func(context.Context) (taskwarrior.Install, error) { return f.Install, f.called("install") },
		Pending: f.pending,
		Linked:  f.linked,
		Add: func(_ context.Context, line string) (string, error) {
			return AddedUUID, f.wrote("add "+line, func() { f.Pending.Tasks = append(f.Pending.Tasks, f.added(line)) })
		},
		Start: func(_ context.Context, uuid string) error {
			return f.wrote("start "+uuid, f.changing(uuid, func(task *taskwarrior.Task) { task.Start = writtenAt() }))
		},
		Stop: func(_ context.Context, uuid string) error {
			return f.wrote("stop "+uuid, f.changing(uuid, func(task *taskwarrior.Task) { task.Start = time.Time{} }))
		},
		Done: func(_ context.Context, uuid string) error { return f.wrote("done "+uuid, f.completing(uuid)) },
		Annotate: func(_ context.Context, uuid, text string) error {
			return f.wrote("annotate "+uuid+" "+text, f.changing(uuid, func(task *taskwarrior.Task) {
				task.Annotations = append(slices.Clone(task.Annotations),
					taskwarrior.Annotation{Entry: writtenAt(), Description: text})
			}))
		},
		Modify: func(_ context.Context, uuid, line string) error { return f.called("modify " + uuid + " " + line) },
		Undo:   func(context.Context) (string, error) { return f.Said, f.called("undo") },
		Sync:   func(context.Context) (string, error) { return f.Said, f.called("sync") },
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

// pending is the pending list as the fake holds it now, a copy no later write
// changes.
func (f *TaskFake) pending(context.Context) (taskwarrior.List, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.record("pending")

	return taskwarrior.List{Tasks: slices.Clone(f.Pending.Tasks), Context: f.Pending.Context}, err
}

// linked is the linked tasks as the fake holds them now, a copy no later
// write changes.
func (f *TaskFake) linked(context.Context) ([]taskwarrior.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.record("linked")

	return slices.Clone(f.Linked), err
}

// wrote records call and, unless Fail names its verb, makes change, both
// under one hold of the lock, answering the error Fail names.
func (f *TaskFake) wrote(call string, change func()) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.record(call)
	if err != nil {
		return err
	}

	change()

	return nil
}

// added is the pending task an add of line makes, numbered after every
// pending task. The lock is held.
func (f *TaskFake) added(line string) taskwarrior.Task {
	highest := 0
	for _, task := range f.Pending.Tasks {
		highest = max(highest, task.ID)
	}

	return taskwarrior.Task{
		UUID: AddedUUID, ID: highest + 1, Description: line, Status: taskwarrior.Pending,
		Entry: writtenAt(), Modified: writtenAt(),
	}
}

// changing is the change of the task uuid names, in the pending list and in
// the linked tasks, wherever it is held, as modified at writtenAt. The lock
// is held when it runs.
func (f *TaskFake) changing(uuid string, change func(task *taskwarrior.Task)) func() {
	return func() {
		for _, tasks := range [][]taskwarrior.Task{f.Pending.Tasks, f.Linked} {
			for index := range tasks {
				if tasks[index].UUID == uuid {
					change(&tasks[index])
					tasks[index].Modified = writtenAt()
				}
			}
		}
	}
}

// completing is the done of the task uuid names: stopped, completed at
// writtenAt and out of the working set, so out of the pending list. The
// lock is held when it runs.
func (f *TaskFake) completing(uuid string) func() {
	completed := f.changing(uuid, func(task *taskwarrior.Task) {
		task.ID, task.Status, task.Start, task.End = 0, taskwarrior.Completed, time.Time{}, writtenAt()
	})

	return func() {
		completed()

		f.Pending.Tasks = slices.DeleteFunc(f.Pending.Tasks, func(task taskwarrior.Task) bool { return task.UUID == uuid })
	}
}

// called records call and answers the error Fail names for its verb.
func (f *TaskFake) called(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.record(call)
}

// record records call and answers the error Fail names for its verb. The lock
// is held.
func (f *TaskFake) record(call string) error {
	f.calls = append(f.calls, call)
	verb, _, _ := strings.Cut(call, " ")

	return f.Fail[verb]
}

// TaskDay is when the fake's tasks were made.
func TaskDay() time.Time {
	return time.Date(2026, time.September, 21, 9, 30, 0, 0, time.UTC)
}

// writtenAt is when the fake makes a write, by its own clock: after every
// date its tasks were made with but their due one.
func writtenAt() time.Time {
	return TaskDay().Add(4 * time.Hour)
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
