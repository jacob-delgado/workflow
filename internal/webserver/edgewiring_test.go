// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/edge"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// These tests hold Handler's wiring of the request boundary, whose own tests
// are internal/webserver/edge's: that the run's dry-run, session and app reach
// it, in front of the server.

func TestWritesAreRefusedInDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := webtest.FilledDeps()
	deps.Git.Checkout = func(context.Context, string) error {
		called = true

		return nil
	}

	dryRun := webserver.Info{Version: webtest.TestVersion, DryRun: true}
	handler := webtest.ServeWith(t, deps, config.Default(), dryRun)

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, webtest.CheckoutPath, `{"branch":"main"}`)

	// Assert
	if recorder.Code != http.StatusForbidden || called {
		t.Errorf("status = %d and checked out %v, want 403 and nothing checked out — read-only in dry-run",
			recorder.Code, called)
	}
}

func TestARefusedWriteChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := webtest.FilledDeps()
	deps.Git.Checkout = func(context.Context, string) error {
		called = true

		return nil
	}

	world := webserver.World{Deps: deps, Config: config.Default()}

	handler, err := webserver.Handler(t.Context(), world, fstest.MapFS{}, edge.NewSession())
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	// Act
	recorder := webtest.Send(t, handler, http.MethodPost, webtest.CheckoutPath, `{"branch":"main"}`)

	// Assert
	if recorder.Code != http.StatusUnauthorized || called {
		t.Errorf("status = %d and checked out %v, want 401 and nothing checked out", recorder.Code, called)
	}
}

// serveApp is Handler with a stand-in app mounted, its index naming cockpit.
func serveApp(t *testing.T) http.Handler {
	t.Helper()

	app := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>cockpit</title>")}}

	return webtest.ServeWorld(t, webserver.World{
		Config: config.Default(),
		Info:   webserver.Info{Version: webtest.TestVersion},
	}, app)
}

func TestTheAppHandedToHandlerIsServed(t *testing.T) {
	t.Parallel()

	// Act
	recorder := webtest.Get(t, serveApp(t), "/")

	// Assert
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("status %d, body %q; want the app's index.html", recorder.Code, recorder.Body.String())
	}
}

func TestTheAPIIsNotShadowedByTheApp(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serveApp(t)

	// Act
	health := webtest.Decode[api.Health](t, webtest.Get(t, handler, "/api/health"))

	// Assert
	if health.Version != webtest.TestVersion {
		t.Errorf("health = %+v, want the API to answer /api even with the app mounted", health)
	}
}
