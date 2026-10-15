// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

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

// EndsWithin is how long a context the server ends is given to be seen ended:
// one it never ends is never seen, so the bound only has to outlast a busy
// machine.
const EndsWithin = 2 * time.Second

// HeldCall is a seam call held until its context ends, or until it is let go:
// it says when it is entered, and how its context stood when it returned.
type HeldCall struct {
	Entered chan struct{}
	ended   chan error
	Release chan struct{}
}

// HeldPatience is how long a call held until its context ends waits before
// giving up, so a context that never ends fails its test rather than holding
// it open: longer than EndsWithin, which the test waits for it.
const HeldPatience = 2 * EndsWithin

// NewHeldUntilEnded is a call held until its context ends, or until patience
// runs out.
func NewHeldUntilEnded() *HeldCall {
	return &HeldCall{Entered: make(chan struct{}, 1), ended: make(chan error, 1), Release: nil}
}

// NewHeld is a call held until it is let go, whatever its context does.
func NewHeld() *HeldCall {
	return &HeldCall{Entered: make(chan struct{}, 1), ended: make(chan error, 1), Release: make(chan struct{})}
}

// Wait is the call's body: it blocks until its context ends, or until it is let
// go for one that is held, and answers how its context stood then.
func (b *HeldCall) Wait(ctx context.Context) error {
	b.Entered <- struct{}{}

	if b.Release != nil {
		<-b.Release
	} else {
		select {
		case <-ctx.Done():
		case <-time.After(HeldPatience):
		}
	}

	err := ctx.Err()
	b.ended <- err

	if err != nil {
		return fmt.Errorf("the held call: %w", err)
	}

	return nil
}

// EndedWith is how the call's context stood when it returned, or a failure
// once patience runs out.
func (b *HeldCall) EndedWith(t *testing.T) error {
	t.Helper()

	select {
	case err := <-b.ended:
		return err
	case <-time.After(EndsWithin):
		t.Fatal("the call never returned")

		return nil
	}
}

// Once is HeldCall's Wait for the first call alone: every later call answers at
// once, as a read made after the server stopped would.
func (b *HeldCall) Once() func(context.Context) error {
	var began atomic.Bool

	return func(ctx context.Context) error {
		if began.CompareAndSwap(false, true) {
			return b.Wait(ctx)
		}

		return nil
	}
}

// Asking sends a request under ctx on its own goroutine, as the HTTP server
// does, and closes the channel it returns once it has been answered.
func Asking(ctx context.Context, handler http.Handler, method, target string) <-chan struct{} {
	request := httptest.NewRequestWithContext(ctx, method, target, nil)
	request.Host = LoopbackHost
	answered := make(chan struct{})

	go func() {
		defer close(answered)

		handler.ServeHTTP(httptest.NewRecorder(), request)
	}()

	return answered
}

// ServeUntilStopped is Serve, and the function that stops the server it
// serves from, ending its lifetime.
func ServeUntilStopped(t *testing.T, deps webserver.Deps) (http.Handler, context.CancelFunc) {
	t.Helper()

	handler, _, stop := ServeForALifetime(t, deps, config.Default())

	return handler, stop
}

// ServeForALifetime is Serve over deps and cfg for a lifetime of its own, which
// webserver.Serve would derive each request's context from, and the function
// that stops the server, ending it.
func ServeForALifetime(
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

	return Presenting(t, session, handler), lifetime, stop
}
