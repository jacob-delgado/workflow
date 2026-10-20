// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"slices"
	"time"

	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskProject is the Taskwarrior project the tool's work is filed under.
const taskProject = "workflow"

// taskDeps is the demo's Taskwarrior: four tasks pending, as the web mock's
// (mockTasks.ts), and one finished yesterday.
func (w *World) taskDeps() seams.Tasks {
	return seams.Tasks{
		Install: func(context.Context) (taskwarrior.Install, error) {
			return taskwarrior.Install{Program: "task", Version: taskwarrior.MinimumVersion, SyncConfigured: true}, nil
		},
		Pending: func(context.Context) (taskwarrior.List, error) {
			return taskwarrior.List{Tasks: taskwarrior.ByUrgency(pendingTasks())}, nil
		},
		Linked: func(context.Context) ([]taskwarrior.Task, error) {
			return slices.DeleteFunc(pendingTasks(), func(task taskwarrior.Task) bool { return task.IssueKey == "" }), nil
		},
		Touched: touchedTasks,
	}
}

// pendingTasks are the tasks still to do: the started one tracking PROJ-412,
// one for another issue of yours, one no issue tracks, and one waiting.
func pendingTasks() []taskwarrior.Task {
	const day = 24 * time.Hour

	start := epoch()

	return []taskwarrior.Task{
		{
			UUID: "7c1e9a42-3b5d-4f60-8a1c-2d4e6f80a1b3", ID: 1, Description: string(workedKey) + ": " + workedSummary,
			Status: taskwarrior.Pending, Project: taskProject, Priority: "H", Tags: []string{taskwarrior.LinkTag},
			Start: start.Add(-72 * time.Minute), Entry: start.Add(-3 * day), Modified: start.Add(-72 * time.Minute),
			Urgency: 14.2, IssueKey: string(workedKey), IssueURL: browseURL(workedKey),
			Annotations: []taskwarrior.Annotation{
				{Entry: start.Add(-26 * time.Hour), Description: "Sam can review it once CI is green"},
			},
		},
		{
			UUID: "2f4a6c8e-0b1d-4e3f-9a5b-7c9d1e3f5a7b", ID: 2,
			Description: "PROJ-408: Cache the forge CI status between polls",
			Status:      taskwarrior.Pending, Project: taskProject, Priority: "M", Tags: []string{taskwarrior.LinkTag},
			Due: start.Add(2 * day), Entry: start.Add(-5 * day), Modified: start.Add(-5 * day),
			Urgency: 9.1, IssueKey: "PROJ-408", IssueURL: browseURL("PROJ-408"),
		},
		{
			UUID: "9e8d7c6b-5a4f-4e3d-8c2b-1a0f9e8d7c6b", ID: 3, Description: "Renew the staging TLS certificate",
			Status: taskwarrior.Pending, Project: "ops", Due: start.Add(day), Entry: start.Add(-2 * day),
			Modified: start.Add(-2 * day), Urgency: 6.3,
		},
		{
			UUID: "4b3a2918-0f7e-4d6c-9b5a-483726150f4e", ID: 4, Description: "Book a room for the sprint retro",
			Status: taskwarrior.Pending, Wait: start.Add(3 * day), Entry: start.Add(-day), Modified: start.Add(-day),
			Urgency: 1.8,
		},
	}
}

// touchedTasks is every task changed since a time.
func touchedTasks(_ context.Context, since time.Time) ([]taskwarrior.Task, error) {
	return changedSince(allTasks(), since), nil
}

// allTasks is every task: the pending ones and the redaction plan written up
// yesterday.
func allTasks() []taskwarrior.Task {
	finished := taskwarrior.Task{
		UUID: "3c8f0a19-e6a8-4544-9dd8-f1888f6f079f", Description: "Write up the token redaction plan",
		Status: taskwarrior.Completed, Project: taskProject, Entry: epoch().Add(-26 * time.Hour),
		Modified: yesterday(9, 40), End: yesterday(9, 40),
	}

	return append(pendingTasks(), finished)
}

// changedSince is the tasks of tasks changed since a time.
func changedSince(tasks []taskwarrior.Task, since time.Time) []taskwarrior.Task {
	return slices.DeleteFunc(tasks, func(task taskwarrior.Task) bool { return task.Modified.Before(since) })
}
