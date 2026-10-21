// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package runs serves the git runs of `workflow --web` and lefthook's hooks:
// POST /api/runs, which starts a hook, a rebase, an amend or a fixup and
// streams it while it goes, DELETE /api/runs/current, which stops it, and the
// hooks lefthook configures and the offer to set it up. It changes with the
// tools it drives, git and lefthook, rather than with the rest of the API, so
// it stands beside the server, which embeds its Runner.
package runs

import (
	"sync"

	"github.com/jacob-delgado/workflow/internal/seams"
)

// Seams is what the runs ask of the world: the repository's git and its
// hooks, as the server's Deps holds them. A nil function means the run, or
// the hook operation, that needs it is not available.
type Seams struct {
	Git   seams.Git
	Hooks seams.Hooks
}

// Runner serves the git runs and the hook operations over its seams, and
// holds the one run going, if any.
type Runner struct {
	seams Seams

	// unexpected hears a failure the answer leaves out.
	unexpected func(err error)

	// indexWrites is the server's queue of index writes, which a run holds
	// while it goes.
	indexWrites *sync.Mutex

	// run is the git run going, if any.
	run runSlot

	// landed tells the server's event streams that a run has ended.
	landed func()
}

// New is a Runner over given that hands a failure the answer leaves out to
// unexpected, holds indexWrites, the lock the server's own index writes take,
// while a run goes, and calls landed once a run's program has ended, before
// its stream says how, so the page reads the end beside a frame that shows it.
func New(given Seams, unexpected func(err error), indexWrites *sync.Mutex, landed func()) *Runner {
	return &Runner{seams: given, unexpected: unexpected, indexWrites: indexWrites, run: runSlot{}, landed: landed}
}

// orZero dereferences an optional field, or returns its zero value when absent.
func orZero[T any](pointer *T) T {
	if pointer == nil {
		var zero T

		return zero
	}

	return *pointer
}
