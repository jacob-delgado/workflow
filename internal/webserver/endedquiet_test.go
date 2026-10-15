// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// The tests here hold a request whose context ended, its page gone or the
// server stopping, to answering without alarm: what it was doing failed with
// its context, and nothing went wrong to hear of, whichever way the server
// would hear of a failure.

func TestARepositoriesReadWhosePageLeftIsNotHandedToUnexpected(t *testing.T) {
	t.Parallel()

	// Arrange
	held := webtest.NewHeldUntilEnded()
	deps, heard := webtest.Hearing(webtest.FilledDeps())
	deps.Store.Favorites = func(ctx context.Context) ([]string, error) { return nil, held.Wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := webtest.Asking(ctx, webtest.Serve(t, deps, config.Default()), http.MethodGet, "/api/repositories")

	<-held.Entered

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
	held := webtest.NewHeldUntilEnded()
	deps, heard := webtest.Hearing(webtest.HookDeps(webtest.OldHooks(), false, &[]hooks.Generated{}))
	deps.Hooks.Write = func(ctx context.Context, _ hooks.Generated) error { return held.Wait(ctx) }
	handler, stop := webtest.ServeUntilStopped(t, deps)
	answered := make(chan struct{})

	go func() {
		defer close(answered)

		webtest.Send(t, handler, http.MethodPost, webtest.HookSetupAt, `{}`)
	}()

	<-held.Entered

	// Act
	stop()

	// Assert
	<-answered

	if len(*heard) != 0 {
		t.Errorf("Unexpected heard %v, want a write the server's stop ended kept quiet", *heard)
	}
}
