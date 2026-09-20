// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestNonLoopbackHostsAreRejected(t *testing.T) {
	t.Parallel()

	handler := serve(t, webserver.Deps{}, config.Default())

	// A foreign name (a rebound hostname) and a foreign IP literal both name
	// somewhere other than the loopback interface.
	for _, host := range []string{"attacker.example.com", "attacker.example.com:7000", "10.0.0.1:7000"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			// Arrange
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)
			request.Host = host
			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != http.StatusForbidden {
				t.Errorf("status = %d for host %q, want 403 — a non-loopback host must be refused", recorder.Code, host)
			}
		})
	}
}

func TestLoopbackHostsReachTheAPI(t *testing.T) {
	t.Parallel()

	handler := serve(t, webserver.Deps{}, config.Default())

	// The loopback name and IP literals, with and without a port.
	for _, host := range []string{"127.0.0.1:7000", "localhost:7000", "[::1]:7000", "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			// Arrange
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)
			request.Host = host
			recorder := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(recorder, request)

			// Assert
			if recorder.Code != http.StatusOK {
				t.Errorf("status = %d for host %q, want 200 — a loopback host must reach the API", recorder.Code, host)
			}
		})
	}
}
