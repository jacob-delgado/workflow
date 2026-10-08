// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// fakeUI stands in for the built single-page app: an index and one asset.
func fakeUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>cockpit</title>")},
		"assets/app.js": {Data: []byte("export const ready = true")},
	}
}

// serveUI builds the handler with ui mounted, so the app is served alongside the
// API. Handler fails only on a spec that cannot load, which is a build defect.
func serveUI(t *testing.T, ui fs.FS) http.Handler {
	t.Helper()

	return serveWorld(t, webserver.World{Config: config.Default(), Info: webserver.Info{Version: testVersion}}, ui)
}

func TestTheAppIsServedAtTheRoot(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serveUI(t, fakeUI()), "/")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("body = %q, want the app's index.html", recorder.Body.String())
	}
}

func TestAStaticAssetIsServedWithItsContent(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serveUI(t, fakeUI()), "/assets/app.js")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "export const ready") {
		t.Errorf("body = %q, want the asset's content", recorder.Body.String())
	}
}

func TestAnAppRouteFallsBackToIndex(t *testing.T) {
	t.Parallel()

	// Act
	// The path names no file, so the client-side router owns it: the server hands
	// back index.html and lets the app route in the browser.
	recorder := get(t, serveUI(t, fakeUI()), "/issues/PROJ-1")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("body = %q, want index.html for an unknown app route", recorder.Body.String())
	}
}

func TestADirectoryFallsBackToIndexRatherThanListing(t *testing.T) {
	t.Parallel()

	// Act
	// assets/ is a directory; it must serve the app, not a listing of the files
	// under it.
	recorder := get(t, serveUI(t, fakeUI()), "/assets")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("body = %q, want index.html rather than a directory listing", recorder.Body.String())
	}
}

func TestAHeadRequestIsServed(t *testing.T) {
	t.Parallel()

	// Act
	recorder := send(t, serveUI(t, fakeUI()), http.MethodHead, "/", "")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 — HEAD is a read the app allows", recorder.Code)
	}
}

func TestTheAPIIsNotShadowedByTheApp(t *testing.T) {
	t.Parallel()

	// Act
	health := decode[api.Health](t, get(t, serveUI(t, fakeUI()), "/api/health"))

	// Assert
	if health.Version != testVersion {
		t.Errorf("health = %+v, want the API to answer /api even with the app mounted", health)
	}
}

func TestTheAppRefusesANonReadMethod(t *testing.T) {
	t.Parallel()

	// Act
	recorder := send(t, serveUI(t, fakeUI()), http.MethodPost, "/issues", "")

	// Assert
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 — the app is read-only", recorder.Code)
	}
}

func TestEveryAnswerRefusesToBeFramedOrSniffed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ host, target string }{
		"the app's root":       {host: loopbackHost, target: "/"},
		"the app's index":      {host: loopbackHost, target: "/index.html"},
		"an asset":             {host: loopbackHost, target: "/assets/app.js"},
		"the API":              {host: loopbackHost, target: healthPath},
		"a refused host's ask": {host: "evil.example:13579", target: "/"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			request.Host = tt.host
			recorder := httptest.NewRecorder()

			// Act
			serveUI(t, fakeUI()).ServeHTTP(recorder, request)

			// Assert
			policy := recorder.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
				if !strings.Contains(policy, directive) {
					t.Errorf("Content-Security-Policy = %q, want it to hold %q", policy, directive)
				}
			}

			for header, want := range map[string]string{
				"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer",
			} {
				if got := recorder.Header().Get(header); got != want {
					t.Errorf("%s = %q, want %q", header, got, want)
				}
			}
		})
	}
}
