// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// marshal renders a configuration as the request body a client would send.
func marshal(t *testing.T, cfg config.Config) string {
	t.Helper()

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshaling the config body: %v", err)
	}

	return string(data)
}

// putConfig saves body as Settings does: it reads the configuration first, and
// saves over the revision that read returned.
func putConfig(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	return putConfigOver(t, handler, body, get(t, handler, "/api/config").Header().Get("ETag"))
}

// putConfigOver saves body over the revision etag names, sent as If-Match; an
// empty etag sends none.
func putConfigOver(t *testing.T, handler http.Handler, body, etag string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/config", strings.NewReader(body))
	request.Host = loopbackHost
	request.Header.Set("Content-Type", "application/json")

	if etag != "" {
		request.Header.Set("If-Match", etag)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

// rewrite leaves the file at path holding contents, as an edit made outside
// the browser does.
func rewrite(t *testing.T, path, contents string) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// taskwarriorProgram is a taskwarrior.program a user might set.
const taskwarriorProgram = "/opt/homebrew/bin/task"
