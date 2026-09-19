// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package webserver serves the local web API behind `workflow --web`: the same
// information the terminal interface shows, and the configuration file, over the
// REST surface described by api/openapi.yaml. It reuses the domain seams the TUI
// and the CLI already use — it is another consumer of the wiring, not a second
// implementation — and writes nothing but the configuration file.
package webserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// Deps is what the server asks of the world, as plain functions over the domain
// clients — the same seams the interface declares, narrowed to what the read and
// configure API needs. A nil function means the service is not configured; the
// handler answers with an empty result rather than an error.
type Deps struct {
	Search   func(jql string, startAt int) (jira.SearchResult, error)
	Issue    func(key jira.Key) (jira.IssueDetail, error)
	Branch   func() (gitrepo.Branch, error)
	Changes  func() ([]gitrepo.Change, error)
	FindPull func(branch string) (forge.PullRequest, bool, error)
	CheckCI  func(pull forge.PullRequest, head string) (forge.CI, error)
	Author   func() (string, error)
}

// Info is the build and run facts the API reports and the server needs.
type Info struct {
	Version string
	DryRun  bool
}

// server implements api.StrictServerInterface over the seams and configuration.
// The configuration is held behind a lock because the write endpoint replaces it
// while read endpoints are serving concurrent requests.
type server struct {
	deps Deps
	info Info

	mu  sync.RWMutex
	cfg config.Config
}

var _ api.StrictServerInterface = (*server)(nil)

// Handler builds the http.Handler that serves the API: the strict typed handlers
// wired to the generated router on a stdlib mux, wrapped in the request validator
// so every request is checked against the contract first. It fails only when the
// embedded spec cannot be loaded, which is a build defect rather than a runtime
// condition.
func Handler(deps Deps, cfg config.Config, info Info) (http.Handler, error) {
	doc, err := loadSpec()
	if err != nil {
		return nil, err
	}

	srv := &server{deps: deps, info: info, cfg: cfg}

	strict := api.NewStrictHandlerWithOptions(srv, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  writeRequestError,
		ResponseErrorHandlerFunc: writeResponseError,
	})

	mux := http.NewServeMux()
	handler := api.HandlerWithOptions(strict, api.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: writeRequestError,
	})

	return validate(doc)(handler), nil
}

// LoopbackAddr is where the server listens in production: the loopback
// interface only, so the API is reachable from this machine and nowhere else.
const LoopbackAddr = "127.0.0.1:7000"

// readHeaderTimeout bounds how long a client may take to send its headers, so a
// slow or stuck connection cannot tie up the server.
const readHeaderTimeout = 10 * time.Second

// shutdownGrace is how long in-flight requests are given to finish when the
// context is canceled before the listener is closed.
const shutdownGrace = 5 * time.Second

// Serve runs handler at addr until ctx is canceled, then drains in-flight
// requests within shutdownGrace and returns. A clean shutdown is not an error.
// Production passes LoopbackAddr; a test passes a loopback address with port 0.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// context.AfterFunc runs the shutdown on the context package's own goroutine
	// when ctx is done, so the server drains without this package starting one.
	// The shutdown uses a fresh context on purpose: ctx is already canceled —
	// that is why we are shutting down — so reusing it would abandon the drain.
	//nolint:contextcheck // the shutdown intentionally detaches from the canceled ctx
	stop := context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()

		_ = srv.Shutdown(shutdownCtx)
	})
	defer stop()

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return fmt.Errorf("serving the web API: %w", err)
}
