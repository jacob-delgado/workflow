// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"cmp"
	"time"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// The uuids of the tasks withTasks holds, each ending in its working-set id so a
// recorded call names the task it was for.
const (
	activeTaskUUID  = "5f1c2b3a-7d4e-4f60-8a9b-000000000012"
	trackedTaskUUID = "5f1c2b3a-7d4e-4f60-8a9b-000000000003"
	looseTaskUUID   = "5f1c2b3a-7d4e-4f60-8a9b-000000000009"
	waitingTaskUUID = "5f1c2b3a-7d4e-4f60-8a9b-000000000015"
	addedTaskUUID   = "5f1c2b3a-7d4e-4f60-8a9b-000000000021"
)

// taskWorld is Taskwarrior, faked. Each field is an answer a test can change
// before the world is used. A world whose tasks is nil is a machine with no task
// program at all, which is every world but withTasks's.
type taskWorld struct {
	install    taskwarrior.Install
	installErr error
	pending    []taskwarrior.Task
	linked     []taskwarrior.Task
	context    string
	// err is the pending list's failure, and linkedErr the linked tasks'.
	err       error
	linkedErr error
	addUUID   string
	writeErr  error
	// annotateErr fails only an annotation, so a task can be added and then not
	// annotated.
	annotateErr error
	// none leaves every seam nil, as wiring does when no task program is found
	// or the integration is turned off.
	none     bool
	undoSaid string
	syncSaid string
}

// withTasks is a world with Taskwarrior holding two pending tasks linked to the
// issues the Issues pane lists — 12 on PROJ-412, started 72 minutes before
// testNow, and 3 on PROJ-388 — one linked to nothing, 9, and one waiting, 15.
// The pending list is given out of urgency order; the fake answers it most
// urgent first, as Taskwarrior's client does.
func withTasks() *world {
	repo := newWorld()

	active := taskwarrior.Task{
		UUID: activeTaskUUID, ID: 12, Description: issueKey + ": " + issueSummary, Status: taskwarrior.Pending,
		Project: "workflow", Tags: []string{taskwarrior.LinkTag}, Start: testNow().Add(-72 * time.Minute),
		Urgency: 14.2, IssueKey: issueKey, IssueURL: "https://jira.example.com/browse/" + issueKey,
		Annotations: []taskwarrior.Annotation{{Entry: testNow().Add(-2 * time.Hour), Description: pullURL}},
	}
	tracked := taskwarrior.Task{
		UUID: trackedTaskUUID, ID: 3, Description: secondIssue + ": Add retries", Status: taskwarrior.Pending,
		Urgency: 8.1, IssueKey: secondIssue, IssueURL: "https://jira.example.com/browse/" + secondIssue,
	}
	loose := taskwarrior.Task{
		UUID: looseTaskUUID, ID: 9, Description: "Renew the cert", Status: taskwarrior.Pending,
		Due: testNow().Add(50 * time.Hour), Urgency: 2,
	}
	waiting := taskwarrior.Task{
		UUID: waitingTaskUUID, ID: 15, Description: "Water the plants", Status: taskwarrior.Pending,
		Wait: testNow().Add(48 * time.Hour), Urgency: 20,
	}

	repo.tasks = &taskWorld{
		install: taskwarrior.Install{Program: "task", Version: taskwarrior.MinimumVersion},
		pending: []taskwarrior.Task{loose, waiting, tracked, active},
		linked:  []taskwarrior.Task{active, tracked},
		addUUID: addedTaskUUID,
	}

	return repo
}

// taskDeps fakes Taskwarrior: no seam at all without one, and otherwise its
// answers, recording each read of the lists and each write. The linked read is
// recorded as "tasks linked", so asked("link") still sees only Jira's links.
func (w *world) taskDeps() seams.Tasks {
	if w.tasks == nil || w.tasks.none {
		return seams.Tasks{}
	}

	fake := w.tasks

	return seams.Tasks{
		Install: func() (taskwarrior.Install, error) { return fake.install, fake.installErr },
		Pending: func() (taskwarrior.List, error) {
			w.record("tasks")

			return taskwarrior.List{Tasks: taskwarrior.ByUrgency(fake.pending), Context: fake.context}, fake.err
		},
		Linked: func() ([]taskwarrior.Task, error) {
			w.record("tasks linked")

			return fake.linked, fake.linkedErr
		},
		Add: func(line string) (string, error) {
			w.record("task add " + line)

			return fake.addUUID, fake.writeErr
		},
		Start: func(uuid string) error { return w.taskWrite("task start " + uuid) },
		Stop:  func(uuid string) error { return w.taskWrite("task stop " + uuid) },
		Done:  func(uuid string) error { return w.taskWrite("task done " + uuid) },
		Annotate: func(uuid, text string) error {
			return cmp.Or(fake.annotateErr, w.taskWrite("task annotate "+uuid+" "+text))
		},
		Modify: func(uuid, line string) error { return w.taskWrite("task modify " + uuid + " " + line) },
		Undo: func() (string, error) {
			w.record("task undo")

			return fake.undoSaid, fake.writeErr
		},
		Sync: func() (string, error) {
			w.record("task sync")

			return fake.syncSaid, fake.writeErr
		},
	}
}

// taskWrite records a write to Taskwarrior and answers it with the world's write
// failure, if it has one.
func (w *world) taskWrite(call string) error {
	w.record(call)

	return w.tasks.writeErr
}
