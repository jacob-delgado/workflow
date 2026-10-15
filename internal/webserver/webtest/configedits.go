// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// Marshal renders a configuration as the request body a client would send.
func Marshal(t *testing.T, cfg config.Config) string {
	t.Helper()

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling the config body: %v", err)
	}

	return string(data)
}

// PutConfig saves body as Settings does: it reads the configuration first, and
// saves over the revision that read returned.
func PutConfig(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return PutConfigOver(t, handler, body, Get(t, handler, "/api/config").Header().Get("ETag"))
}

// PutConfigOver saves body over the revision etag names, sent as If-Match; an
// empty etag sends none.
func PutConfigOver(t *testing.T, handler http.Handler, body, etag string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/config", strings.NewReader(body))
	request.Host = LoopbackHost
	request.Header.Set("Content-Type", "application/json")

	if etag != "" {
		request.Header.Set("If-Match", etag)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// Rewrite leaves the file at path holding contents, as an edit made outside
// the browser does.
func Rewrite(t *testing.T, path, contents string) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// TaskwarriorProgram is a taskwarrior.program a user might set.
const TaskwarriorProgram = "/opt/homebrew/bin/task"
