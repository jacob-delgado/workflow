// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package edge_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

func TestABadQueryParameterIsRejected(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}

	// Act
	recorder := get(t, serveOver(t, ops), "/api/issues?start_at=notanumber")

	// Assert
	if recorder.Code != http.StatusBadRequest || ops.reached.Load() != 0 {
		t.Errorf("status = %d and reached %d times, want 400 and none for an unparsable parameter",
			recorder.Code, ops.reached.Load())
	}
}

func TestAMalformedBodyIsRejected(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}

	// Act
	recorder := call{method: http.MethodPut, target: configPath, body: "{not json"}.send(t, serveOver(t, ops))

	// Assert
	if recorder.Code != http.StatusBadRequest || ops.reached.Load() != 0 {
		t.Errorf("status = %d and reached %d times, want 400 and none for a malformed body",
			recorder.Code, ops.reached.Load())
	}

	if got := decode[api.Problem](t, recorder).Code; got != api.ProblemCodeBadRequest {
		t.Errorf("code = %q, want bad_request", got)
	}
}

func TestAnUnknownEndpointIsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serveOver(t, &operations{}), "/api/nonexistent")

	// Assert
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an unknown endpoint", recorder.Code)
	}

	failure := decode[api.Problem](t, recorder)
	if failure.Code != api.ProblemCodeNotFound {
		t.Errorf("code = %q, want not_found", failure.Code)
	}
}

func TestAWriteTheContractRoutesNowhereIsNotFound(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serveOver(t, &operations{})

	// Act
	recorder := call{method: http.MethodPost, target: "/api/no-such-operation", body: `{}`}.send(t, handler)

	// Assert
	if failure := decode[api.Problem](t, recorder); recorder.Code != http.StatusNotFound ||
		failure.Code != api.ProblemCodeNotFound {
		t.Errorf("status/code %d/%s, want 404/not_found", recorder.Code, failure.Code)
	}
}

func TestAWrongMethodOnAKnownPathIsNotAllowed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method    string
		target    string
		wantAllow string
	}{
		"a write to a read":              {method: http.MethodPost, target: branchPath, wantAllow: "GET"},
		"a method no operation declares": {method: http.MethodDelete, target: configPath, wantAllow: "GET, PUT"},
		"a read of a templated write":    {method: http.MethodGet, target: "/api/issues/PROJ-412/link", wantAllow: "POST"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := call{method: tt.method, target: tt.target}.send(t, serveOver(t, &operations{}))

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

func TestTheServerIsToldWhichOperationsSwitch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method, target, body string
		want                 bool
	}{
		"a switch of directory": {
			method: http.MethodPut, target: "/api/repositories/here", body: `{"dir":"/elsewhere"}`, want: true,
		},
		"a checkout": {method: http.MethodPost, target: checkoutPath, body: `{"branch":"main"}`, want: false},
		"a read":     {method: http.MethodGet, target: branchPath, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ops := &operations{}
			ops.switched.Store(!tt.want)

			// Act
			recorder := call{method: tt.method, target: tt.target, body: tt.body}.send(t, serveOver(t, ops))

			// Assert
			if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
				t.Fatalf("status = %d and reached %d times, want 200 once; body %q",
					recorder.Code, ops.reached.Load(), recorder.Body.String())
			}

			if got := ops.switched.Load(); got != tt.want {
				t.Errorf("switches = %v, want %v", got, tt.want)
			}
		})
	}
}
