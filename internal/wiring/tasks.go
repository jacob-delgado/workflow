// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskDeps is what a surface asks of Taskwarrior — nothing at all when the
// integration is disabled or no task program can be found, so the Tasks pane
// says so and offers nothing. Which task program it is — go-task is also
// called task — is settled by the first call that finds Taskwarrior, and kept;
// until then every call looks again, so a Taskwarrior just set up is found.
func taskDeps(ctx context.Context, settings config.Taskwarrior) seams.Tasks {
	if settings.Disabled {
		return seams.Tasks{}
	}

	candidates := taskwarrior.Candidates(os.Getenv("PATH"), runtime.GOOS)
	if settings.Program == "" && len(candidates) == 0 {
		return seams.Tasks{}
	}

	install := onceConnected(func() (taskwarrior.Install, error) {
		return taskwarrior.Detect(ctx, settings.Program, candidates, proc.CaptureWithin)
	})

	return taskSeams(ctx, install)
}

// taskSeams binds each seam to the Taskwarrior install finds, asking it first
// so a program that is not Taskwarrior answers every call with why.
func taskSeams(ctx context.Context, install func() (taskwarrior.Install, error)) seams.Tasks {
	client := func() (taskwarrior.Client, error) {
		found, err := install()
		if err != nil {
			return taskwarrior.Client{}, err
		}

		return taskwarrior.New(proc.CaptureWithin, found), nil
	}

	return seams.Tasks{
		Install: install,
		Pending: func() (taskwarrior.List, error) {
			return ask(client, func(tasks taskwarrior.Client) (taskwarrior.List, error) { return tasks.Pending(ctx) })
		},
		Touched: func(since time.Time) ([]taskwarrior.Task, error) {
			return ask(client, func(tasks taskwarrior.Client) ([]taskwarrior.Task, error) {
				return tasks.Touched(ctx, since)
			})
		},
		Linked: func() ([]taskwarrior.Task, error) {
			return ask(client, func(tasks taskwarrior.Client) ([]taskwarrior.Task, error) {
				return tasks.Linked(ctx)
			})
		},
		Add: func(line string) (string, error) {
			return ask(client, func(tasks taskwarrior.Client) (string, error) { return tasks.Add(ctx, line) })
		},
		Start: func(uuid string) error {
			return tell(client, func(tasks taskwarrior.Client) error { return tasks.Start(ctx, uuid) })
		},
		Stop: func(uuid string) error {
			return tell(client, func(tasks taskwarrior.Client) error { return tasks.Stop(ctx, uuid) })
		},
		Done: func(uuid string) error {
			return tell(client, func(tasks taskwarrior.Client) error { return tasks.Done(ctx, uuid) })
		},
		Annotate: func(uuid, text string) error {
			return tell(client, func(tasks taskwarrior.Client) error { return tasks.Annotate(ctx, uuid, text) })
		},
		Modify: func(uuid, line string) error {
			return tell(client, func(tasks taskwarrior.Client) error { return tasks.Modify(ctx, uuid, line) })
		},
		Undo: func() (string, error) {
			return ask(client, func(tasks taskwarrior.Client) (string, error) { return tasks.Undo(ctx) })
		},
		Sync: func() (string, error) {
			return ask(client, func(tasks taskwarrior.Client) (string, error) { return tasks.Sync(ctx) })
		},
	}
}
