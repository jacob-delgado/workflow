// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
)

// HereHeader is the header a page sends on a write naming the directory it
// shows, escaped as a URL path is, so a write from a page that has not noticed a switch is refused
// rather than made in the directory switched to.
const HereHeader = "Workflow-Here"

var (
	// errWriteInFlight is a switch asked for while a write is being made,
	// which the switch would leave unknown.
	errWriteInFlight = errors.New("a write is in flight; switch once it has finished")
	// errNoSwitching is a server that was given no way to reach another
	// directory.
	errNoSwitching = errors.New("switching directory is not available here")
)

// World is the server's view of one directory: its seams, the configuration
// read there, and the facts of the run there.
type World struct {
	Deps   Deps
	Config config.Config
	Info   Info
}

// worlds holds the server for the directory worked in, and replaces it with
// another directory's on a switch. Every write but a switch holds gate
// shared, so a switch, which holds it alone, is refused while one runs rather
// than leaving it unknown.
type worlds struct {
	// lifetime is the server's own: it ends when the server stops, and every
	// write's context ends with it rather than with the write's page.
	lifetime context.Context //nolint:containedctx // the server's lifetime, which a write outlasts its page to end with

	gate sync.RWMutex

	mu      sync.RWMutex
	current *server
	handler http.Handler

	// setups lets one first-run setup run at a time, so a second waits for
	// the first to write and take its file up, then finds a file applies.
	setups sync.Mutex
}

// newWorlds serves first, for as long as lifetime lasts.
func newWorlds(lifetime context.Context, first World) (*worlds, error) {
	held := &worlds{lifetime: lifetime}

	_, err := held.install(first)
	if err != nil {
		return nil, err
	}

	return held, nil
}

// serve hands a request the contract admitted to the server for the
// directory worked in. A write runs under a context that outlasts its page,
// and a write but a switch holds the gate a switch waits on; a write is
// refused when the page that sent it names another directory.
func (w *worlds) serve(writer http.ResponseWriter, request *http.Request, switches bool) {
	if !isSafeMethod(request.Method) {
		var release context.CancelFunc

		request, release = w.outlivingItsPage(request)
		defer release()
	}

	if !isSafeMethod(request.Method) && !switches {
		w.gate.RLock()
		defer w.gate.RUnlock()
	}

	srv, handler := w.served()

	if !isSafeMethod(request.Method) && showsAnother(request, srv.deps.Repositories.Here.Dir) {
		writeProblem(writer, api.ProblemCodeConflict, "the page shows another directory than the server works in; reload it")

		return
	}

	handler.ServeHTTP(writer, request)
}

// outlivingItsPage is request under a context that goes on when its page
// leaves and ends when the server stops, and the function that releases it
// once the write is answered.
func (w *worlds) outlivingItsPage(request *http.Request) (*http.Request, context.CancelFunc) {
	// Trade-off TRADE-37: a write outlasts its page, and ends only with the
	// server.
	ctx, release := w.outliving(request.Context())

	return request.WithContext(ctx), release
}

// outliving is a context with ctx's values that goes on when ctx ends and ends
// when the server stops, and the function that releases it once what it was
// made for is done. Every request's context derives from the server's
// lifetime, so detaching one from its page alone, never from the server, is
// what lets Serve's drain end on a stop rather than wait out its grace.
func (w *worlds) outliving(ctx context.Context) (context.Context, context.CancelFunc) {
	outlives, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := w.atEnd(cancel)

	return outlives, func() {
		stop()
		cancel()
	}
}

// atEnd runs end, on the context package's own goroutine, once the server's
// lifetime ends, unless the function it returns is called first.
func (w *worlds) atEnd(end func()) func() bool {
	return context.AfterFunc(w.lifetime, end)
}

// install builds the server for world and serves it from now on, retiring the
// one before it so its event streams end and the page reconnects.
func (w *worlds) install(world World) (*server, error) {
	inEffect, seen, err := startingPoint(world.Config)
	if err != nil {
		return nil, fmt.Errorf("reading the configuration file: %w", err)
	}

	srv := &server{
		deps: world.Deps, info: world.Info, files: world.Config.Layers(), cfg: inEffect, seen: seen,
		forgeKind: world.Info.ForgeKind, worlds: w, retired: make(chan struct{}),
	}

	strict := api.NewStrictHandlerWithOptions(srv, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  writeRequestError,
		ResponseErrorHandlerFunc: srv.writeResponseError,
	})

	apiMux := http.NewServeMux()

	// The event stream and a run's output are streaming responses the strict,
	// one-response-object interface cannot express, so they are registered by
	// hand rather than generated.
	apiMux.HandleFunc("GET /api/events", srv.streamEvents)
	apiMux.HandleFunc("POST /api/runs", srv.startRun)

	apiHandler := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter:       apiMux,
		ErrorHandlerFunc: writeRequestError,
	})

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.current != nil {
		close(w.current.retired)
	}

	w.current, w.handler = srv, apiHandler

	return srv, nil
}

// served is the server for the directory worked in, and its handler.
func (w *worlds) served() (*server, http.Handler) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.current, w.handler
}

// showsAnother reports a request whose page names a directory, escaped as a
// URL path is, other than here. A header carries bytes rather than text, so
// a directory named outside ASCII arrives escaped; one that does not
// unescape is not the directory here either.
func showsAnother(request *http.Request, here string) bool {
	escaped := request.Header.Get(HereHeader)
	if escaped == "" {
		return false
	}

	shown, err := url.PathUnescape(escaped)

	return err != nil || shown != here
}

// switchTo wires dir as the first directory was wired, under ctx, and serves
// it from now on, refusing while a write is in flight.
func (w *worlds) switchTo(ctx context.Context, dir string) (*server, error) {
	if !w.gate.TryLock() {
		return nil, errWriteInFlight
	}
	defer w.gate.Unlock()

	current, _ := w.served()

	reach := current.deps.Reach
	if reach == nil {
		return nil, errNoSwitching
	}

	world, err := reach(ctx, dir)
	if err != nil {
		return nil, err
	}

	// Where the server's failures are heard stays where it was; what they are
	// masked with is the configuration in effect, handed over at each one.
	if world.Deps.Unexpected == nil {
		world.Deps.Unexpected = current.deps.Unexpected
	}

	return w.install(world)
}
