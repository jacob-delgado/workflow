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
)

// The tests here hold the context each kind of request hands git: a read's
// ends with its page, and a frame's with its stream.

func TestAChangesReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeldUntilEnded(), filledDeps()
	deps.Git.Changes = func(ctx context.Context) ([]gitrepo.Change, error) { return nil, held.wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, deps, config.Default()), http.MethodGet, "/api/changes")

	<-held.entered

	// Act
	leave()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered
}

func TestAStreamsBranchReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, deps := newHeldUntilEnded(), filledDeps()
	deps.Git.Branch = func(ctx context.Context) (gitrepo.Branch, error) { return gitrepo.Branch{}, held.wait(ctx) }
	stream := openStream(t, serve(t, deps, config.Default()), "/api/events")

	<-held.entered

	// Act
	stream.cancel()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the frame's read ended with %v, want it canceled once its page left", err)
	}
}
