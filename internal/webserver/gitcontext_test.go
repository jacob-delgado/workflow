// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The tests here hold the context each kind of request hands git: a read's
// ends with its page, and a frame's with its stream.

func TestAChangesReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := webtest.NewHeldUntilEnded(), webtest.FilledDeps()
	deps.Git.Changes = func(ctx context.Context) ([]gitrepo.Change, error) { return nil, held.Wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := webtest.Asking(ctx, webtest.Serve(t, deps, config.Default()), http.MethodGet, "/api/changes")

	<-held.Entered

	// Act
	leave()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered
}

func TestAStreamsBranchReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := webtest.NewHeldUntilEnded(), webtest.FilledDeps()
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, held.Wait(ctx) }
	stream := webtest.OpenStream(t, webtest.Serve(t, deps, config.Default()), "/api/events")

	<-held.Entered

	// Act
	stream.Cancel()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the frame's read ended with %v, want it canceled once its page left", err)
	}
}

func TestARepositoriesReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := webtest.NewHeldUntilEnded(), webtest.FilledDeps()
	deps.Repositories.Worktrees = func(ctx context.Context) ([]gitrepo.Worktree, error) { return nil, held.Wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := webtest.Asking(ctx, webtest.Serve(t, deps, config.Default()), http.MethodGet, "/api/repositories")

	<-held.Entered

	// Act
	leave()

	// Assert
	err := held.EndedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered
}
