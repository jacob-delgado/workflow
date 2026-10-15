// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/edge"
)

// Serve builds the API handler over deps and cfg, not in dry-run, so the write
// endpoints are reachable — the common case — with deps' Taskwarrior bound
// with cfg's settings, as at start. A test that exercises dry-run's read-only
// guard passes its own Info. Handler fails only when the embedded spec cannot
// load, which is a build defect, so the test fails there.
func Serve(t *testing.T, deps webserver.Deps, cfg config.Config) http.Handler {
	t.Helper()

	return ServeWith(t, deps, cfg, webserver.Info{Version: TestVersion, Taskwarrior: cfg.Taskwarrior})
}

// ServeWith is Serve with the caller's Info, for the tests that need a specific
// stream interval.
func ServeWith(t *testing.T, deps webserver.Deps, cfg config.Config, info webserver.Info) http.Handler {
	t.Helper()

	return ServeWorld(t, webserver.World{Deps: deps, Config: cfg, Info: info}, fstest.MapFS{})
}

// ServeWorld builds the handler for world with assets mounted, under a session
// of its own that it presents on each request as the page does. Handler fails
// only on a spec that cannot load, which is a build defect.
func ServeWorld(t *testing.T, world webserver.World, assets fs.FS) http.Handler {
	t.Helper()

	session := edge.NewSession()

	handler, err := webserver.Handler(t.Context(), world, assets, session)
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	return Presenting(t, session, handler)
}

// Presenting hands handler each request with session presented as the page
// presents it, unless the request presents one itself, as a test of the
// session's check does.
func Presenting(t *testing.T, session edge.Session, handler http.Handler) http.Handler {
	t.Helper()

	authorization := "Bearer " + SessionToken(t, session)

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			request.Header.Set("Authorization", authorization)
		}

		handler.ServeHTTP(w, request)
	})
}

// SessionToken is the session the address the server prints carries, read
// from its fragment as the page reads it.
func SessionToken(t *testing.T, session edge.Session) string {
	t.Helper()

	address, err := url.Parse(session.Address(LoopbackHost))
	if err != nil {
		t.Fatalf("reading the session's address: %v", err)
	}

	token, found := strings.CutPrefix(address.Fragment, "session=")
	if !found || token == "" {
		t.Fatalf("address %s carries no session in its fragment", address)
	}

	return token
}

// Get sends a GET and returns the recorder.
func Get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	return Send(t, handler, http.MethodGet, target, "")
}

// Send sends a request with an optional body and returns the recorder.
func Send(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequestWithContext(t.Context(), method, target, reader)
	request.Host = LoopbackHost

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// ProblemJSON is the media type every refusal is written in.
const ProblemJSON = "application/problem+json"

// Decode unmarshals the recorder's JSON body into T, failing the test on error.
func Decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T

	err := json.Unmarshal(recorder.Body.Bytes(), &value)
	if err != nil {
		t.Fatalf("decoding %T from %q: %v", value, recorder.Body.String(), err)
	}

	return value
}
