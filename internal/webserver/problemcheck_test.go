// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
)

// problemJSON is the media type every refusal is written in.
const problemJSON = "application/problem+json"

// assertProblem checks that a refusal is a problem of status whose detail
// says want.
func assertProblem(t *testing.T, recorder *httptest.ResponseRecorder, status int, want string) api.Problem {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, status, recorder.Body.String())
	}

	if kind := recorder.Header().Get("Content-Type"); kind != problemJSON {
		t.Errorf("Content-Type = %q, want application/problem+json", kind)
	}

	failure := decode[api.Problem](t, recorder)
	if !strings.Contains(failure.Detail, want) {
		t.Errorf("detail = %q, want it to say %q", failure.Detail, want)
	}

	return failure
}
