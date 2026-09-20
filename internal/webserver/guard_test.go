// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// postCheckout sends a checkout write with the given Origin header, so a test can
// exercise the cross-origin write guard.
func postCheckout(t *testing.T, handler http.Handler, origin string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(
		t.Context(), http.MethodPost, "/api/checkout", strings.NewReader(`{"branch":"fix/PROJ-1"}`),
	)
	request.Host = loopbackHost
	request.Header.Set("Content-Type", "application/json")

	if origin != "" {
		request.Header.Set("Origin", origin)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

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

func TestACrossOriginWriteIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())

	// Act
	recorder := postCheckout(t, handler, "http://evil.example.com")

	// Assert
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a write from a foreign origin", recorder.Code)
	}
}

func TestAMalformedOriginIsRefusedForAWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())

	// Act
	recorder := postCheckout(t, handler, "http://%zz")

	// Assert
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a write whose origin does not parse", recorder.Code)
	}
}

func TestASafeMethodSkipsTheWriteGuard(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())
	request := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/api/checkout", nil)
	request.Host = loopbackHost
	request.Header.Set("Origin", "http://evil.example.com")

	recorder := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(recorder, request)

	// Assert
	// A safe method is exempt from the cross-origin write guard, so even a foreign
	// origin is not refused by it.
	if recorder.Code == http.StatusForbidden {
		t.Error("a safe method was refused by the cross-origin write guard")
	}
}

func TestASameOriginWriteIsNotRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serve(t, filledDeps(), config.Default())

	// Act
	// A loopback Origin passes the guard and reaches the handler; filledDeps has a
	// dirty tree, so the checkout is refused with 409 — not the guard's 403.
	recorder := postCheckout(t, handler, "http://127.0.0.1:7000")

	// Assert
	if recorder.Code == http.StatusForbidden {
		t.Errorf("status = %d, want the guard to allow a same-origin write", recorder.Code)
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
