// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The reads the session's tests ask for: one answered from the seams, and the
// server's own health.
const (
	branchPath = "/api/branch"
	healthPath = "/api/health"
)

// serveBare builds the handler over filled seams under session, with nothing
// in between that presents it for a request, as a program that was never
// handed the page's address would ask.
func serveBare(t *testing.T, session webserver.Session) http.Handler {
	t.Helper()

	world := webserver.World{Deps: filledDeps(), Config: config.Default(), Info: webserver.Info{Version: testVersion}}

	handler, err := webserver.Handler(world, fstest.MapFS{}, session)
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	return handler
}

// sessionRequest is a request to the API presenting authorization, when it is
// not empty; one to the event stream is canceled from the start, so the stream
// pushes one frame and ends.
type sessionRequest struct{ method, target, body, authorization string }

// send serves the request through handler.
func (r sessionRequest) send(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	if strings.HasPrefix(r.target, "/api/events") {
		cancel()
	}
	defer cancel()

	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}

	request := httptest.NewRequestWithContext(ctx, r.method, r.target, body)
	request.Host = loopbackHost
	request.Header.Set("Content-Type", "application/json")

	if r.authorization != "" {
		request.Header.Set("Authorization", r.authorization)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestARequestWithoutTheSessionIsRefused(t *testing.T) {
	t.Parallel()

	session := webserver.NewSession()
	handler := serveBare(t, session)
	token := sessionToken(t, session)

	cases := map[string]sessionRequest{
		"a read presenting none":     {method: http.MethodGet, target: branchPath},
		"a write presenting none":    {method: http.MethodPut, target: "/api/config", body: `{}`},
		"the health presenting none": {method: http.MethodGet, target: healthPath},
		"a read presenting another run's": {
			method: http.MethodGet, target: branchPath, authorization: "Bearer " + sessionToken(t, webserver.NewSession()),
		},
		"a read presenting it in another scheme": {
			method: http.MethodGet, target: branchPath, authorization: "Basic " + token,
		},
		"a read presenting it in its query, which only the stream takes": {
			method: http.MethodGet, target: branchPath + "?session=" + token,
		},
		"the stream presenting none": {method: http.MethodGet, target: "/api/events"},
		"the stream presenting another run's in its query": {
			method: http.MethodGet, target: "/api/events?session=" + sessionToken(t, webserver.NewSession()),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := tt.send(t, handler)

			// Assert
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body %q", recorder.Code, recorder.Body.String())
			}

			if got := decode[api.Problem](t, recorder).Code; got != api.ProblemCodeUnauthorized {
				t.Errorf("code = %q, want unauthorized", got)
			}

			if got := recorder.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Bearer") {
				t.Errorf("WWW-Authenticate = %q, want the Bearer scheme named", got)
			}
		})
	}
}

func TestARefusedWriteChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	deps := filledDeps()
	deps.Checkout = func(string) error {
		called = true

		return nil
	}

	handler, err := webserver.Handler(webserver.World{Deps: deps, Config: config.Default()}, fstest.MapFS{},
		webserver.NewSession())
	if err != nil {
		t.Fatalf("building the handler: %v", err)
	}

	// Act
	recorder := sessionRequest{method: http.MethodPost, target: "/api/checkout", body: `{"branch":"main"}`}.
		send(t, handler)

	// Assert
	if recorder.Code != http.StatusUnauthorized || called {
		t.Errorf("status = %d and checked out %v, want 401 and nothing checked out", recorder.Code, called)
	}
}

func TestTheSessionAdmitsARequestPresentingIt(t *testing.T) {
	t.Parallel()

	session := webserver.NewSession()
	handler := serveBare(t, session)
	token := sessionToken(t, session)

	cases := map[string]sessionRequest{
		"a read presenting it as a bearer": {
			method: http.MethodGet, target: branchPath, authorization: "Bearer " + token,
		},
		"a read naming the scheme in lowercase": {
			method: http.MethodGet, target: healthPath, authorization: "bearer " + token,
		},
		"the stream presenting it in its query": {method: http.MethodGet, target: "/api/events?session=" + token},
		"the stream presenting it as a bearer": {
			method: http.MethodGet, target: "/api/events", authorization: "Bearer " + token,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := tt.send(t, handler)

			// Assert
			if recorder.Code != http.StatusOK {
				t.Errorf("status = %d, want 200; body %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTheSessionsAddressCarriesItInItsFragmentAlone(t *testing.T) {
	t.Parallel()

	// Act
	address, err := url.Parse(webserver.NewSession().Address(loopbackHost))
	// Assert
	if err != nil {
		t.Fatalf("parsing the address: %v", err)
	}

	if address.Scheme != "http" || address.Host != loopbackHost || address.Path != "/" || address.RawQuery != "" {
		t.Errorf("address = %s, want the page at http://%s/ with no query", address, loopbackHost)
	}

	// rand.Text's 26 characters carry 128 bits.
	const shortest = 26
	if token, found := strings.CutPrefix(address.Fragment, "session="); !found || len(token) < shortest {
		t.Errorf("fragment = %q, want session= and a token of at least %d characters", address.Fragment, shortest)
	}
}

func TestEachRunMakesASessionOfItsOwn(t *testing.T) {
	t.Parallel()

	// Act
	first, second := webserver.NewSession(), webserver.NewSession()

	// Assert
	if sessionToken(t, first) == sessionToken(t, second) {
		t.Error("two sessions carry the same token, want each its own")
	}
}

func TestTheZeroSessionAdmitsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serveBare(t, webserver.Session{})

	// Act
	recorder := sessionRequest{method: http.MethodGet, target: healthPath, authorization: "Bearer "}.send(t, handler)

	// Assert
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", recorder.Code)
	}
}
