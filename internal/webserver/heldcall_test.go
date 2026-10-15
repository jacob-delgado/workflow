// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// endsWithin is how long a context the server ends is given to be seen ended:
// one it never ends is never seen, so the bound only has to outlast a busy
// machine.
const endsWithin = 2 * time.Second

// heldCall is a seam call held until its context ends, or until it is let go:
// it says when it is entered, and how its context stood when it returned.
type heldCall struct {
	entered chan struct{}
	ended   chan error
	release chan struct{}
}

// heldPatience is how long a call held until its context ends waits before
// giving up, so a context that never ends fails its test rather than holding
// it open: longer than endsWithin, which the test waits for it.
const heldPatience = 2 * endsWithin

// newHeldUntilEnded is a call held until its context ends, or until patience
// runs out.
func newHeldUntilEnded() *heldCall {
	return &heldCall{entered: make(chan struct{}, 1), ended: make(chan error, 1), release: nil}
}

// newHeld is a call held until it is let go, whatever its context does.
func newHeld() *heldCall {
	return &heldCall{entered: make(chan struct{}, 1), ended: make(chan error, 1), release: make(chan struct{})}
}

// wait is the call's body: it blocks until its context ends, or until it is let
// go for one that is held, and answers how its context stood then.
func (b *heldCall) wait(ctx context.Context) error {
	b.entered <- struct{}{}

	if b.release != nil {
		<-b.release
	} else {
		select {
		case <-ctx.Done():
		case <-time.After(heldPatience):
		}
	}

	err := ctx.Err()
	b.ended <- err

	if err != nil {
		return fmt.Errorf("the held call: %w", err)
	}

	return nil
}

// endedWith is how the call's context stood when it returned, or a failure
// once patience runs out.
func (b *heldCall) endedWith(t *testing.T) error {
	t.Helper()

	select {
	case err := <-b.ended:
		return err
	case <-time.After(endsWithin):
		t.Fatal("the call never returned")

		return nil
	}
}

// once is held's wait for the first call alone: every later call answers at
// once, as a read made after the server stopped would.
func (b *heldCall) once() func(context.Context) error {
	var began atomic.Bool

	return func(ctx context.Context) error {
		if began.CompareAndSwap(false, true) {
			return b.wait(ctx)
		}

		return nil
	}
}

// asking sends a request under ctx on its own goroutine, as the HTTP server
// does, and closes the channel it returns once it has been answered.
func asking(ctx context.Context, handler http.Handler, method, target string) <-chan struct{} {
	request := httptest.NewRequestWithContext(ctx, method, target, nil)
	request.Host = loopbackHost
	answered := make(chan struct{})

	go func() {
		defer close(answered)

		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	return answered
}

// serveUntilStopped is serve, and the function that stops the server it
// serves from, ending its lifetime.
func serveUntilStopped(t *testing.T, deps webserver.Deps) (http.Handler, context.CancelFunc) {
	t.Helper()

	handler, _, stop := serveForALifetime(t, deps, config.Default())

	return handler, stop
}

// serveForALifetime is serve over deps and cfg for a lifetime of its own, which
// Serve would derive each request's context from, and the function that stops
// the server, ending it.
func serveForALifetime(
	t *testing.T, deps webserver.Deps, cfg config.Config,
) (http.Handler, context.Context, context.CancelFunc) {
	t.Helper()

	lifetime, stop := context.WithCancel(t.Context())
	session := webserver.NewSession()
	world := webserver.World{Deps: deps, Config: cfg, Info: webserver.Info{Taskwarrior: cfg.Taskwarrior}}

	handler, err := webserver.Handler(lifetime, world, fstest.MapFS{}, session)
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	return presenting(t, session, handler), lifetime, stop
}
