// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package edge_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/webserver/edge"
)

func TestARequestWithoutTheSessionIsRefused(t *testing.T) {
	t.Parallel()

	session := edge.NewSession()
	token := sessionToken(t, session)

	cases := map[string]call{
		"a read presenting none":     {method: http.MethodGet, target: branchPath},
		"a write presenting none":    {method: http.MethodPut, target: configPath, body: `{}`},
		"the health presenting none": {method: http.MethodGet, target: healthPath},
		"a read presenting another run's": {
			method: http.MethodGet, target: branchPath, authorization: "Bearer " + sessionToken(t,
				edge.NewSession()),
		},
		"a read presenting it in another scheme": {
			method: http.MethodGet, target: branchPath, authorization: "Basic " + token,
		},
		"a read presenting it in its query, which only the stream takes": {
			method: http.MethodGet, target: branchPath + "?session=" + token,
		},
		"the stream presenting none": {method: http.MethodGet, target: "/api/events"},
		"the stream presenting another run's in its query": {
			method: http.MethodGet, target: "/api/events?session=" + sessionToken(t, edge.NewSession()),
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ops := &operations{}
			handler := serveBare(t, session, ops)

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

			if ops.reached.Load() != 0 {
				t.Error("the refused request reached the server")
			}
		})
	}
}

func TestTheSessionAdmitsARequestPresentingIt(t *testing.T) {
	t.Parallel()

	session := edge.NewSession()
	token := sessionToken(t, session)

	cases := map[string]call{
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

			// Arrange
			ops := &operations{}
			handler := serveBare(t, session, ops)

			// Act
			recorder := tt.send(t, handler)

			// Assert
			if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
				t.Errorf("status = %d and reached %d times, want 200 once; body %q",
					recorder.Code, ops.reached.Load(), recorder.Body.String())
			}
		})
	}
}

func TestTheSessionsAddressCarriesItInItsFragmentAlone(t *testing.T) {
	t.Parallel()

	// Act
	address, err := url.Parse(edge.NewSession().Address(loopbackHost))
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
	first, second := edge.NewSession(), edge.NewSession()

	// Assert
	if sessionToken(t, first) == sessionToken(t, second) {
		t.Error("two sessions carry the same token, want each its own")
	}
}

func TestTheZeroSessionAdmitsNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}
	handler := serveBare(t, edge.Session{}, ops)

	// Act
	recorder := call{method: http.MethodGet, target: healthPath, authorization: "Bearer "}.send(t, handler)

	// Assert
	if recorder.Code != http.StatusUnauthorized || ops.reached.Load() != 0 {
		t.Errorf("status = %d and reached %d times, want 401 and none", recorder.Code, ops.reached.Load())
	}
}
