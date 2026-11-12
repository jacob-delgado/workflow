// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// goingRun is a run's output that writes nothing and goes until end is
// closed, then passes.
func goingRun(end chan string) proc.Output {
	return proc.Output{Lines: end, Wait: func() error { return nil }, Stop: func() {}}
}

func TestAStageWaitsForTheRunGoing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The run and a stage share the server's index queue: a stage sent while
	// the run's program goes is held back until the run ends, as two stages
	// are.
	var staged atomic.Bool

	started, end := make(chan struct{}), make(chan string)
	configuredHooks := hooks.Configured{Runnable: []string{webtest.PreCommitHook}}

	deps := webtest.FilledDeps()
	deps.Git.Stage = func(context.Context, gitrepo.Change) error {
		staged.Store(true)

		return nil
	}
	deps.Hooks.Configured = func(context.Context) (hooks.Configured, error) { return configuredHooks, nil }
	deps.Hooks.Run = func(context.Context, string) (proc.Output, error) {
		close(started)

		return goingRun(end), nil
	}
	handler := webtest.Serve(t, deps, config.Default())

	var (
		writes       sync.WaitGroup
		ran, staging int
	)

	writes.Go(func() {
		ran = webtest.Send(t, handler, http.MethodPost, "/api/runs", `{"kind":"hook","hook":"pre-commit"}`).Code
	})

	<-started

	// Act
	writes.Go(func() { staging = webtest.Send(t, handler, http.MethodPost, stagePath, everything).Code })
	time.Sleep(indexHeld)

	stagedWhileRan := staged.Load()

	close(end)
	writes.Wait()

	// Assert
	if ran != http.StatusOK || staging != http.StatusOK || !staged.Load() {
		t.Fatalf("run status = %d, stage status = %d, staged %t; want both 200, staged", ran, staging, staged.Load())
	}

	if stagedWhileRan {
		t.Error("a stage went while the run's program held the index, want it to wait for the run")
	}
}

func TestACheckoutWaitsForTheRunGoing(t *testing.T) {
	t.Parallel()

	// Arrange
	// A hook or a rebase works over the tree it started on, which may be
	// clean: a checkout sent while the run's program goes is held back until
	// the run ends, as a stage is.
	var going, checkedOutWhileRan, checkedOut atomic.Bool

	started, end := make(chan struct{}), make(chan string)
	configuredHooks := hooks.Configured{Runnable: []string{webtest.PreCommitHook}}

	deps := webtest.FilledDeps()
	deps.Git.Changes = func(context.Context) ([]gitrepo.Change, error) { return nil, nil }
	deps.Git.Checkout = func(context.Context, string) error {
		checkedOutWhileRan.Store(going.Load())
		checkedOut.Store(true)

		return nil
	}
	deps.Hooks.Configured = func(context.Context) (hooks.Configured, error) { return configuredHooks, nil }
	deps.Hooks.Run = func(context.Context, string) (proc.Output, error) {
		going.Store(true)
		close(started)

		return goingRun(end), nil
	}
	handler := webtest.Serve(t, deps, config.Default())

	var (
		writes        sync.WaitGroup
		ran, checking int
	)

	writes.Go(func() {
		ran = webtest.Send(t, handler, http.MethodPost, "/api/runs", `{"kind":"hook","hook":"pre-commit"}`).Code
	})

	<-started

	// Act
	writes.Go(func() {
		checking = webtest.Send(t, handler, http.MethodPost, "/api/checkout", `{"branch":"`+webtest.TestBase+`"}`).Code
	})
	time.Sleep(indexHeld)

	going.Store(false)
	close(end)
	writes.Wait()

	// Assert
	if ran != http.StatusOK || checking != http.StatusOK || !checkedOut.Load() {
		t.Fatalf("run status = %d, checkout status = %d, checked out %t; want both 200, checked out",
			ran, checking, checkedOut.Load())
	}

	if checkedOutWhileRan.Load() {
		t.Error("a checkout went while the run's program held the index, want it to wait for the run")
	}
}
