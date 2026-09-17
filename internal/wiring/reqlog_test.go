// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/wiring"
)

// errTransport is what a Doer answers when a request never reaches the service.
var errTransport = errors.New("dial tcp: timeout")

// steppingClock advances by step on each call, so a request's start time and
// its duration are deterministic.
func steppingClock(step time.Duration) func() time.Time {
	base := time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC)
	calls := 0

	return func() time.Time {
		now := base.Add(time.Duration(calls) * step)
		calls++

		return now
	}
}

// okDoer answers every request with 200 and an empty body.
func okDoer(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

// request builds a request or fails the test.
func request(t *testing.T, method, url string) *http.Request {
	t.Helper()

	built, err := http.NewRequest(method, url, nil) //nolint:noctx // a fixture request needs no context
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}

	return built
}

func TestRequestLogRecordsTheOutlineAndNoSecret(t *testing.T) {
	t.Parallel()

	// Arrange
	var out strings.Builder

	log := wiring.NewRequestLog(&out, steppingClock(150*time.Millisecond))
	wrapped := log.Wrap("jira", okDoer)

	secretURL := "https://alice:pw@jira.example.com/rest/api/2/search?jql=secret&token=hunter2"
	req := request(t, http.MethodPost, secretURL)
	req.Header.Set("Authorization", "Bearer super-secret-token")

	// Act
	_, _ = wrapped(req)

	// Assert
	line := out.String()
	for _, want := range []string{"jira", "POST", "/rest/api/2/search", "200", "150ms"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q is missing %q", line, want)
		}
	}

	for _, secret := range []string{"super-secret-token", "hunter2", "jql", "alice", "jira.example.com"} {
		if strings.Contains(line, secret) {
			t.Errorf("log %q leaked %q", line, secret)
		}
	}
}

func TestRequestLogRedactsAWebhookPath(t *testing.T) {
	t.Parallel()

	// Arrange
	var out strings.Builder

	log := wiring.NewRequestLog(&out, steppingClock(10*time.Millisecond))
	wrapped := log.Wrap("slack", okDoer)

	// Act
	_, _ = wrapped(request(t, http.MethodPost, "https://hooks.slack.com/services/T0/B0/XXXXSECRET"))

	// Assert
	line := out.String()
	if strings.Contains(line, "XXXXSECRET") || strings.Contains(line, "/T0/B0") {
		t.Errorf("log leaked the webhook secret: %q", line)
	}

	if !strings.Contains(line, "/services") {
		t.Errorf("log should still note that a webhook was posted to: %q", line)
	}
}

func TestRequestLogRecordsARequestThatNeverAnswered(t *testing.T) {
	t.Parallel()

	// Arrange
	var out strings.Builder

	log := wiring.NewRequestLog(&out, steppingClock(5*time.Millisecond))

	failing := func(*http.Request) (*http.Response, error) { return nil, errTransport }
	wrapped := log.Wrap("forge", failing)

	// Act
	_, _ = wrapped(request(t, http.MethodGet, "https://api.github.com/repos/x/y"))

	// Assert
	line := out.String()
	if !strings.Contains(line, "/repos/x/y") || !strings.Contains(line, "GET") {
		t.Errorf("a failed request was not recorded: %q", line)
	}
}

func TestANilRequestLogWrapsToTheSameDoer(t *testing.T) {
	t.Parallel()

	// Arrange
	var log *wiring.RequestLog

	called := false
	responder := func(*http.Request) (*http.Response, error) {
		called = true

		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	}

	// Act
	wrapped := log.Wrap("jira", responder)
	_, _ = wrapped(request(t, http.MethodGet, "https://x/y"))

	// Assert
	if !called {
		t.Error("a nil log's wrap did not call through to the Doer")
	}
}
