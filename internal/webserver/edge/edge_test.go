// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package edge_test

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/webserver/edge"
)

// The paths these tests ask for: two reads, one answered from the seams and
// the server's own health, a write, and the configuration, read and written.
const (
	branchPath   = "/api/branch"
	healthPath   = "/api/health"
	checkoutPath = "/api/checkout"
	configPath   = "/api/config"
)

// loopbackHost is the Host the requests here send, so they pass the loopback
// guard. These tests build no server, so they share none of webtest's
// fixtures, which would link the whole of internal/webserver into them.
const loopbackHost = "127.0.0.1:13579"

// operations stands in for the server behind the boundary: it answers every
// request handed to it 200, counting them, and keeps whether the contract
// marked the last one's operation a switch.
type operations struct {
	reached  atomic.Int32
	switched atomic.Bool
}

// serve is an edge.OperationServer.
func (o *operations) serve(writer http.ResponseWriter, _ *http.Request, switches bool) {
	o.reached.Add(1)
	o.switched.Store(switches)
	writer.WriteHeader(http.StatusOK)
}

// fakeUI stands in for the built single-page app: an index and one asset.
func fakeUI() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>cockpit</title>")},
		"assets/app.js": {Data: []byte("export const ready = true")},
	}
}

// build is the boundary around ops, with the fake app mounted, under session.
// Handler fails only on a spec that cannot load, which is a build defect.
func build(t *testing.T, dryRun bool, session edge.Session, assets fs.FS, ops *operations) http.Handler {
	t.Helper()

	handler, err := edge.Handler(dryRun, session, assets, ops.serve)
	if err != nil {
		t.Fatalf("building the boundary: %v", err)
	}

	return handler
}

// serveOver is the boundary around ops, not in dry-run, under a session of its
// own that it presents on each request as the page does.
func serveOver(t *testing.T, ops *operations) http.Handler {
	t.Helper()

	return serveApp(t, fakeUI(), ops)
}

// serveApp is serveOver with assets as the app.
func serveApp(t *testing.T, assets fs.FS, ops *operations) http.Handler {
	t.Helper()

	session := edge.NewSession()

	return presenting(t, session, build(t, false, session, assets, ops))
}

// serveReadOnly is serveOver under --dry-run.
func serveReadOnly(t *testing.T, ops *operations) http.Handler {
	t.Helper()

	session := edge.NewSession()

	return presenting(t, session, build(t, true, session, fakeUI(), ops))
}

// serveBare is the boundary around ops under session, with nothing in between
// that presents it for a request, as a program that was never handed the
// page's address would ask.
func serveBare(t *testing.T, session edge.Session, ops *operations) http.Handler {
	t.Helper()

	return build(t, false, session, fakeUI(), ops)
}

// presenting hands handler each request with session presented as the page
// presents it, unless the request presents one itself, as a test of the
// session's check does.
func presenting(t *testing.T, session edge.Session, handler http.Handler) http.Handler {
	t.Helper()

	authorization := "Bearer " + sessionToken(t, session)

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "" {
			request.Header.Set("Authorization", authorization)
		}

		handler.ServeHTTP(w, request)
	})
}

// sessionToken is the session the address the server prints carries, read
// from its fragment as the page reads it.
func sessionToken(t *testing.T, session edge.Session) string {
	t.Helper()

	address, err := url.Parse(session.Address(loopbackHost))
	if err != nil {
		t.Fatalf("reading the session's address: %v", err)
	}

	token, found := strings.CutPrefix(address.Fragment, "session=")
	if !found || token == "" {
		t.Fatalf("address %s carries no session in its fragment", address)
	}

	return token
}

// decode unmarshals the recorder's JSON body into T, failing the test on error.
func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T

	err := json.Unmarshal(recorder.Body.Bytes(), &value)
	if err != nil {
		t.Fatalf("decoding %T from %q: %v", value, recorder.Body.String(), err)
	}

	return value
}

// call is one request to the boundary: from loopbackHost unless host names
// another, carrying origin and authorization as their headers when they are
// not empty, and body as JSON when it is not.
type call struct{ method, target, body, host, origin, authorization string }

// send serves the request through handler and returns the recorder.
func (c call) send(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if c.body != "" {
		body = strings.NewReader(c.body)
	}

	request := httptest.NewRequestWithContext(t.Context(), c.method, c.target, body)
	request.Host = loopbackHost

	if c.host != "" {
		request.Host = c.host
	}

	if c.body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	for header, value := range map[string]string{"Origin": c.origin, "Authorization": c.authorization} {
		if value != "" {
			request.Header.Set(header, value)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// get sends a GET from the loopback host and returns the recorder.
func get(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	return call{method: http.MethodGet, target: target}.send(t, handler)
}

// checkout is a checkout write carrying origin, so a test can exercise the
// cross-origin write guard.
func checkout(origin string) call {
	return call{method: http.MethodPost, target: checkoutPath, body: `{"branch":"fix/PROJ-1"}`, origin: origin}
}
