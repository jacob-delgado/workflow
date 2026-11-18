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

// errDiskFull and errLogClosed are what a log's writer answers when a line
// cannot be written.
var (
	errDiskFull  = errors.New("no space left on device")
	errLogClosed = errors.New("file already closed")
)

// scriptedWriter answers each write with the next of its errors, and writes a
// line through to written when that error is nil or the errors have run out.
type scriptedWriter struct {
	errs    []error
	written strings.Builder
}

func (w *scriptedWriter) Write(line []byte) (int, error) {
	if len(w.errs) > 0 {
		err := w.errs[0]
		w.errs = w.errs[1:]

		if err != nil {
			return 0, err
		}
	}

	w.written.Write(line)

	return len(line), nil
}

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

func TestRequestLogKeepsItsFirstFailedWriteAndGoesOnWriting(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &scriptedWriter{errs: []error{errDiskFull, nil, errLogClosed}}
	log := wiring.NewRequestLog(out, steppingClock(time.Millisecond))
	wrapped := log.Wrap("jira", okDoer)

	// Act
	for _, path := range []string{"/first", "/second", "/third"} {
		_, _ = wrapped(request(t, http.MethodGet, "https://jira.example.com"+path))
	}

	// Assert
	err := log.Close()
	if !errors.Is(err, errDiskFull) || errors.Is(err, errLogClosed) {
		t.Errorf("Close() = %v, want the first failed write's %q alone", err, errDiskFull)
	}

	if written := out.written.String(); !strings.Contains(written, "/second") {
		t.Errorf("the log stopped writing after a write failed: %q", written)
	}
}

func TestARequestLogClosedWithNothingInFlightAndNothingFailedIsClean(t *testing.T) {
	t.Parallel()

	// Arrange
	var out strings.Builder

	log := wiring.NewRequestLog(&out, steppingClock(time.Millisecond))
	_, _ = log.Wrap("jira", okDoer)(request(t, http.MethodGet, "https://jira.example.com/rest/api/2/myself"))

	// Act
	err := log.Close()

	// Assert
	if err != nil || !strings.Contains(out.String(), "/rest/api/2/myself") {
		t.Errorf("Close() = %v, the log holding %q; want nil, and the outline written", err, out.String())
	}
}

// closedUnderARequest is log's Close, called while a request it wraps is
// out, once that request has landed after it.
func closedUnderARequest(t *testing.T, log *wiring.RequestLog) error {
	t.Helper()

	arrived, release, answered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	held := log.Wrap("slack", func(*http.Request) (*http.Response, error) {
		close(arrived)
		<-release

		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	late := request(t, http.MethodPost, "https://slack.com/api/oauth.v2.access")

	go func() {
		defer close(answered)

		_, _ = held(late)
	}()

	<-arrived

	err := log.Close()

	close(release)
	<-answered

	return err
}

func TestARequestStillInFlightWhenTheLogClosesIsCountedNotWritten(t *testing.T) {
	t.Parallel()

	// Arrange
	var out strings.Builder

	log := wiring.NewRequestLog(&out, steppingClock(time.Millisecond))

	// Act
	err := closedUnderARequest(t, log)

	// Assert
	if !errors.Is(err, wiring.ErrRequestsUnrecorded) || !strings.HasSuffix(err.Error(), ": 1") {
		t.Errorf("Close() = %v, want %q, counting the one request in flight", err, wiring.ErrRequestsUnrecorded)
	}

	if out.Len() != 0 {
		t.Errorf("the log holds %q, want no outline written after it closed", out.String())
	}
}

func TestAnEchoedLogStillSaysARequestThatLandsAfterItCloses(t *testing.T) {
	t.Parallel()

	// Arrange
	var file, echo strings.Builder

	log := wiring.NewEchoedRequestLog(&file, &echo, steppingClock(time.Millisecond))

	// Act
	_ = closedUnderARequest(t, log)

	// Assert
	if file.Len() != 0 || !strings.Contains(echo.String(), "slack POST /api/oauth.v2.access 200") {
		t.Errorf("the file holds %q and the echo %q; want the late outline echoed alone", file.String(),
			echo.String())
	}
}
