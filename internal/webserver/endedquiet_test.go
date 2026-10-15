// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
)

// The tests here hold a request whose context ended, its page gone or the
// server stopping, to answering without alarm: what it was doing failed with
// its context, and nothing went wrong to hear of, whichever way the server
// would hear of a failure.

func TestARepositoriesReadWhosePageLeftIsNotHandedToUnexpected(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	deps, heard := hearing(filledDeps())
	deps.Store.Favorites = func(ctx context.Context) ([]string, error) { return nil, held.wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, deps, config.Default()), http.MethodGet, "/api/repositories")

	<-held.entered

	// Act
	leave()

	// Assert
	<-answered

	if len(*heard) != 0 {
		t.Errorf("Unexpected heard %v, want a read its page left kept quiet", *heard)
	}
}

func TestAHookSetupTheServerStoppedIsNotHandedToUnexpected(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	deps, heard := hearing(hookDeps(oldHooks(), false, &[]hooks.Generated{}))
	deps.Hooks.Write = func(ctx context.Context, _ hooks.Generated) error { return held.wait(ctx) }
	handler, stop := serveUntilStopped(t, deps)
	answered := make(chan struct{})

	go func() {
		defer close(answered)

		send(t, handler, http.MethodPost, hookSetupAt, `{}`)
	}()

	<-held.entered

	// Act
	stop()

	// Assert
	<-answered

	if len(*heard) != 0 {
		t.Errorf("Unexpected heard %v, want a write the server's stop ended kept quiet", *heard)
	}
}
