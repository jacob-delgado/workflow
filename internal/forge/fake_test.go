// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

// unusableBase is an API address url.Parse refuses.
const unusableBase = "https://api.example.com/\x7f"

// recorded is what a fake forge saw of one request: its method, escaped path
// and raw query, its headers, and its JSON body decoded.
type recorded struct {
	method, path, query string
	header              http.Header
	body                map[string]any
}

// serveForge starts a forge API that answers with handler, and returns a
// client pointed at it.
func serveForge(t *testing.T, handler http.HandlerFunc) forge.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return forge.New(server.Client().Do, server.URL, secret)
}

// writeJSON answers with status and body, as JSON.
func writeJSON(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(body))
}

// recordingForge answers each request with the status and JSON body answer
// chooses from what was asked, and records every request.
func recordingForge(t *testing.T, answer func(asked recorded) (int, string)) (forge.Client, *[]recorded) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []recorded
	)

	note := func(asked recorded) {
		lock.Lock()
		defer lock.Unlock()

		seen = append(seen, asked)
	}

	client := serveForge(t, func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any

		_ = json.NewDecoder(request.Body).Decode(&body)
		asked := recorded{
			method: request.Method, path: request.URL.EscapedPath(), query: request.URL.RawQuery,
			header: request.Header.Clone(), body: body,
		}
		note(asked)

		status, text := answer(asked)
		writeJSON(writer, status, text)
	})

	return client, &seen
}

// answering answers every request with status and body.
func answering(status int, body string) func(recorded) (int, string) {
	return func(recorded) (int, string) { return status, body }
}

// routing answers each path its mapped body, and a path it does not know with
// an empty list, which reads as "nothing there" rather than an error.
func routing(routes map[string]string) func(recorded) (int, string) {
	return func(asked recorded) (int, string) {
		body, ok := routes[asked.path]
		if !ok {
			body = "[]"
		}

		return http.StatusOK, body
	}
}

// conversation answers each path its mapped body, and a path it does not know
// with an empty object. A path in fails answers 403 instead, to try a step an
// under-scoped token cannot make.
func conversation(answers map[string]string, fails map[string]bool) func(recorded) (int, string) {
	return func(asked recorded) (int, string) {
		if fails[asked.path] {
			return http.StatusForbidden, `{"message":"Resource not accessible by personal access token"}`
		}

		answer, ok := answers[asked.path]
		if !ok {
			answer = "{}"
		}

		return http.StatusOK, answer
	}
}

// lastRequest is what the fake forge last saw.
func lastRequest(t *testing.T, seen *[]recorded) recorded {
	t.Helper()

	if len(*seen) == 0 {
		t.Fatal("the forge was never asked anything")
	}

	return (*seen)[len(*seen)-1]
}

// requestTo is the first recorded request to a path, or a zero request when none
// reached it.
func requestTo(seen []recorded, path string) recorded {
	for _, request := range seen {
		if request.path == path {
			return request
		}
	}

	return recorded{}
}

// requestsTo counts the recorded requests to a path.
func requestsTo(seen []recorded, path string) int {
	count := 0

	for _, asked := range seen {
		if asked.path == path {
			count++
		}
	}

	return count
}
