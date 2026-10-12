// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The tests here hold the context each kind of request hands Taskwarrior: a
// read's ends with its page, a write's with the server, and a search every
// stream shares with neither.

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

// startPath starts the plain task.
const startPath = tasksPath + "/" + plainUUID + "/start"

func TestATaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.wait(ctx) }

	var heard []error

	deps := webserver.Deps{Tasks: tasks, Unexpected: func(_ config.Config, err error) { heard = append(heard, err) }}
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, deps, config.Default()), http.MethodGet, tasksPath)

	<-held.entered

	// Act
	leave()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the read's context ended with %v, want it canceled once its page left", err)
	}

	<-answered

	if len(heard) != 0 {
		t.Errorf("Unexpected heard %v, want a read its page left kept quiet", heard)
	}
}

func TestATaskWriteGoesOnWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeld()
	tasks := fakeTaskwarrior().seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.wait(ctx) }
	ctx, leave := context.WithCancel(t.Context())
	answered := asking(ctx, serve(t, webserver.Deps{Tasks: tasks}, config.Default()), http.MethodPost, startPath)

	<-held.entered

	// Act
	leave()
	close(held.release)

	// Assert
	err := held.endedWith(t)
	if err != nil {
		t.Errorf("the write's context ended with %v once its page left, want it to go on", err)
	}

	<-answered
}

func TestATaskWriteEndsWhenTheServerStops(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Start = func(ctx context.Context, _ string) error { return held.wait(ctx) }
	handler, stop := serveUntilStopped(t, webserver.Deps{Tasks: tasks})
	answered := asking(t.Context(), handler, http.MethodPost, startPath)

	<-held.entered

	// Act
	stop()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the write's context ended with %v, want it canceled once the server stopped", err)
	}

	<-answered
}

func TestAStreamsTaskReadStopsWhenItsPageLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held := newHeldUntilEnded()
	tasks := fakeTaskwarrior().seams()
	tasks.Pending = func(ctx context.Context) (taskwarrior.List, error) { return taskwarrior.List{}, held.wait(ctx) }
	stream := openStream(t, serve(t, webserver.Deps{Tasks: tasks}, config.Default()), "/api/events")

	<-held.entered

	// Act
	stream.cancel()

	// Assert
	err := held.endedWith(t)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the frame's read ended with %v, want it canceled once its page left", err)
	}
}

// Every stream shares the search for Taskwarrior, and holds a failed one for a
// minute: a search canceled because the page that began it left would be held
// as no Taskwarrior for every other page.
func TestASearchForTaskwarriorGoesOnWhenThePageThatBeganItLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	held, fake := newHeld(), fakeTaskwarrior()
	tasks := fake.seams()
	searched := false
	tasks.Install = func(ctx context.Context) (taskwarrior.Install, error) {
		if searched {
			return fake.install, nil
		}

		searched = true

		return fake.install, held.wait(ctx)
	}
	handler := serve(t, webserver.Deps{Tasks: tasks}, config.Default())
	first := openStream(t, handler, "/api/events")

	<-held.entered
	first.cancel()
	close(held.release)
	<-first.done

	// Act
	frame := firstSnapshot(t, streamOnce(t, handler, "/api/events").Body.String())

	// Assert
	if !frame.Tasks.Available {
		t.Errorf("the next page's tasks = %+v, want Taskwarrior found though the first page left mid-search",
			frame.Tasks)
	}
}
