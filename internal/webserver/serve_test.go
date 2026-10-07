// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestServeStopsWhenTheContextIsCanceled(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	handler := serve(t, webserver.Deps{}, config.Default())
	done := make(chan error, 1)

	listener := listen(t)

	go func() { done <- webserver.Serve(ctx, listener, handler) }()

	// Act
	cancel()

	// Assert
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil on a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop when the context was canceled")
	}
}

func TestServeReportsAListenerThatCannotAccept(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, webserver.Deps{}, config.Default())
	listener := listen(t)
	_ = listener.Close()

	// Act
	err := webserver.Serve(context.Background(), listener, handler)

	// Assert
	if err == nil {
		t.Error("Serve returned nil, want the closed listener's failure")
	}
}

// listen is a loopback listener on a port the system chooses, closed with the
// test.
func listen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on loopback: %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	return listener
}

func TestABadQueryParameterIsRejected(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/issues?start_at=notanumber")

	// Assert
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unparsable parameter", recorder.Code)
	}
}

func TestAMalformedBodyIsRejected(t *testing.T) {
	t.Parallel()

	// Act
	recorder := send(t, serve(t, webserver.Deps{}, config.Default()), http.MethodPut, "/api/config", "{not json")

	// Assert
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a malformed body", recorder.Code)
	}
}

func TestAnUnknownEndpointIsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/nonexistent")

	// Assert
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unknown endpoint", recorder.Code)
	}

	failure := decode[api.Problem](t, recorder)
	if failure.Code != api.ProblemCodeNotFound {
		t.Errorf("code = %q, want not_found", failure.Code)
	}
}

func TestAWrongMethodOnAKnownPathIsNotAllowed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method    string
		target    string
		wantAllow string
	}{
		"a write to a read":              {method: http.MethodPost, target: "/api/branch", wantAllow: "GET"},
		"a method no operation declares": {method: http.MethodDelete, target: "/api/config", wantAllow: "GET, PUT"},
		"a read of a templated write":    {method: http.MethodGet, target: "/api/issues/PROJ-412/link", wantAllow: "POST"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := send(t, serve(t, filledDeps(), config.Default()), tt.method, tt.target, "")

			// Assert
			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s: status = %d, want 405 for a method the path does not answer",
					tt.method, tt.target, recorder.Code)
			}

			if allow := recorder.Header().Get("Allow"); allow != tt.wantAllow {
				t.Errorf("Allow = %q, want %q", allow, tt.wantAllow)
			}

			failure := decode[api.Problem](t, recorder)
			if failure.Code != api.ProblemCodeMethodNotAllowed || failure.Status != http.StatusMethodNotAllowed {
				t.Errorf("problem = %s (%d), want method_not_allowed (405)", failure.Code, failure.Status)
			}
		})
	}
}
