// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package edge_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTheAppIsServedAtTheRoot(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serveOver(t, &operations{}), "/")

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
	recorder := get(t, serveOver(t, &operations{}), "/assets/app.js")

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
	recorder := get(t, serveOver(t, &operations{}), "/issues/PROJ-1")

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
	recorder := get(t, serveOver(t, &operations{}), "/assets")

	// Assert
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("body = %q, want index.html rather than a directory listing", recorder.Body.String())
	}
}

func TestTheRootOfAnAppWithNoIndexIsNotListed(t *testing.T) {
	t.Parallel()

	// Arrange
	noIndex := fstest.MapFS{"assets/app.js": {Data: []byte("export const ready = true")}}
	handler := serveApp(t, noIndex, &operations{})

	// Act
	recorder := get(t, handler, "/")

	// Assert
	// The root names no file, so it falls back to the index, which is not
	// there; the file server must not list what the app holds instead.
	if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "assets") {
		t.Errorf("status = %d, body %q; want 404 and no listing of the app's files", recorder.Code, recorder.Body.String())
	}
}

func TestAHeadRequestIsServed(t *testing.T) {
	t.Parallel()

	// Act
	recorder := call{method: http.MethodHead, target: "/"}.send(t, serveOver(t, &operations{}))

	// Assert
	if recorder.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 — HEAD is a read the app allows", recorder.Code)
	}
}

func TestAnAPIPathGoesToTheServerNotTheApp(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}

	// Act
	recorder := get(t, serveOver(t, ops), healthPath)

	// Assert
	if ops.reached.Load() != 1 || strings.Contains(recorder.Body.String(), "cockpit") {
		t.Errorf("reached %d times with body %q, want /api answered by the server even with the app mounted",
			ops.reached.Load(), recorder.Body.String())
	}
}

func TestTheAppRefusesANonReadMethod(t *testing.T) {
	t.Parallel()

	// Act
	recorder := call{method: http.MethodPost, target: "/issues"}.send(t, serveOver(t, &operations{}))

	// Assert
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 — the app is read-only", recorder.Code)
	}

	if allow := recorder.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("Allow = %q, want GET, HEAD", allow)
	}
}
