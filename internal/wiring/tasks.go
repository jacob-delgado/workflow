// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"runtime"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// taskDeps is what a surface asks of Taskwarrior — nothing at all when the
// integration is disabled or no task program can be found, so the Tasks pane
// says so and offers nothing. Which task program it is — go-task is also
// called task — is settled by the first call that finds Taskwarrior, and kept;
// until then every call looks again, so a Taskwarrior just set up is found.
func (e Environment) taskDeps(settings config.Taskwarrior) seams.Tasks {
	if settings.Disabled {
		return seams.Tasks{}
	}

	candidates := taskwarrior.Candidates(e.Getenv("PATH"), runtime.GOOS)
	if settings.Program == "" && len(candidates) == 0 {
		return seams.Tasks{}
	}

	install := onceConnected(func(ctx context.Context) (taskwarrior.Install, error) {
		return taskwarrior.Detect(ctx, settings.Program, candidates, e.CaptureWithin)
	})

	return e.taskSeams(install)
}

// taskSeams binds each seam to the Taskwarrior install finds, asking it first
// so a program that is not Taskwarrior answers every call with why.
func (e Environment) taskSeams(install func(context.Context) (taskwarrior.Install, error)) seams.Tasks {
	client := func(ctx context.Context) (taskwarrior.Client, error) {
		found, err := install(ctx)
		if err != nil {
			return taskwarrior.Client{}, err
		}

		return taskwarrior.New(e.CaptureWithin, found), nil
	}

	return seams.Tasks{
		Install: install,
		Pending: func(ctx context.Context) (taskwarrior.List, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) (taskwarrior.List, error) { return tasks.Pending(ctx) })
		},
		Touched: func(ctx context.Context, since time.Time) ([]taskwarrior.Task, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) ([]taskwarrior.Task, error) {
				return tasks.Touched(ctx, since)
			})
		},
		Linked: func(ctx context.Context) ([]taskwarrior.Task, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) ([]taskwarrior.Task, error) {
				return tasks.Linked(ctx)
			})
		},
		Add: func(ctx context.Context, line string) (string, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) (string, error) { return tasks.Add(ctx, line) })
		},
		Start: func(ctx context.Context, uuid string) error {
			return tell(ctx, client, func(tasks taskwarrior.Client) error { return tasks.Start(ctx, uuid) })
		},
		Stop: func(ctx context.Context, uuid string) error {
			return tell(ctx, client, func(tasks taskwarrior.Client) error { return tasks.Stop(ctx, uuid) })
		},
		Done: func(ctx context.Context, uuid string) error {
			return tell(ctx, client, func(tasks taskwarrior.Client) error { return tasks.Done(ctx, uuid) })
		},
		Annotate: func(ctx context.Context, uuid, text string) error {
			return tell(ctx, client, func(tasks taskwarrior.Client) error { return tasks.Annotate(ctx, uuid, text) })
		},
		Modify: func(ctx context.Context, uuid, line string) error {
			return tell(ctx, client, func(tasks taskwarrior.Client) error { return tasks.Modify(ctx, uuid, line) })
		},
		Undo: func(ctx context.Context) (string, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) (string, error) { return tasks.Undo(ctx) })
		},
		Sync: func(ctx context.Context) (string, error) {
			return ask(ctx, client, func(tasks taskwarrior.Client) (string, error) { return tasks.Sync(ctx) })
		},
	}
}
