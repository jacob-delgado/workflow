// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// webhookPrefix is the start of a Slack incoming-webhook path, whose rest is
// the credential. A webhook is posted through wrapWebhook, which records no
// path at all; Wrap still cuts this one short, should a webhook reach it.
const webhookPrefix = "/services/"

// redacted stands in for the part of a path that is a credential.
const redacted = "[redacted]"

// ErrRequestsUnrecorded is requests still in flight as the request log
// closed, whose outlines it does not hold.
var ErrRequestsUnrecorded = errors.New("requests still in flight as the request log closed, so not in it")

// RequestLog records the outline of each HTTP request — its service, method,
// path, status and duration — for a bug report. It records nothing else: never a
// header, a body, a query string or a host, and never the path of a messaging
// webhook, which is the credential itself; so no credential reaches the file.
// Once closed it writes nothing, and counts what was still in flight; only its
// echo, which outlives it, still says each outline.
type RequestLog struct {
	mu   sync.Mutex
	out  io.Writer
	echo io.Writer
	now  func() time.Time
	// stage is whether out still takes outlines.
	stage logStage
	// inFlight is how many requests it wraps are out, unanswered.
	inFlight int
	writeErr error
}

// logStage is whether a request log still records.
type logStage int

const (
	// logRecording is a log that writes each outline as its request lands.
	logRecording logStage = iota
	// logClosed is a log whose writer is done with: an outline that lands
	// now writes nothing.
	logClosed
)

// NewRequestLog writes outlines to out, timestamped by now. A nil now means
// time.Now.
func NewRequestLog(out io.Writer, now func() time.Time) *RequestLog {
	return NewEchoedRequestLog(out, io.Discard, now)
}

// NewEchoedRequestLog is NewRequestLog with each outline said to echo too:
// every one, whatever out does with it, and before and after the log closes,
// since echo is the debug lines on stderr, which outlive the log's file.
// Echo's own failures go unheard, as every write to stderr's does.
func NewEchoedRequestLog(out, echo io.Writer, now func() time.Time) *RequestLog {
	if now == nil {
		now = time.Now
	}

	return &RequestLog{out: out, echo: echo, now: now}
}

// Wrap decorates a Doer so each request it makes is recorded under service. A
// nil log wraps to the same Doer, so a caller need not branch on whether logging
// is on.
func (l *RequestLog) Wrap(service string, next func(*http.Request) (*http.Response, error),
) func(*http.Request) (*http.Response, error) {
	//nolint:bodyclose // wrap only relays the response; the caller's client reads and closes its body.
	return l.wrap(service, next, func(request *http.Request) string { return loggedPath(request.URL.Path) })
}

// Close stops the log, before its writer closes: an outline that lands from
// now on writes nothing. Its error joins why the first outline the log could
// not write failed, since a failed write does not stop the log, with how many
// requests were still in flight, whose outlines it will not hold; it is nil
// when every outline was written and none is out.
func (l *RequestLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.stage = logClosed

	var unrecorded error
	if l.inFlight > 0 {
		unrecorded = fmt.Errorf("%w: %d", ErrRequestsUnrecorded, l.inFlight)
	}

	return errors.Join(l.writeErr, unrecorded)
}

// wrapWebhook is Wrap for a Doer that posts to a messaging webhook. Every
// kind's webhook path — Slack's, Teams', Discord's or a plain one — is its
// credential, whatever its shape, so the whole path is recorded as redacted.
func (l *RequestLog) wrapWebhook(service string, next func(*http.Request) (*http.Response, error),
) func(*http.Request) (*http.Response, error) {
	//nolint:bodyclose // wrap only relays the response; the caller's client reads and closes its body.
	return l.wrap(service, next, func(*http.Request) string { return redacted })
}

// wrap decorates next, recording each request under service with the path
// safePath gives for it.
func (l *RequestLog) wrap(service string, next func(*http.Request) (*http.Response, error),
	safePath func(*http.Request) string,
) func(*http.Request) (*http.Response, error) {
	if l == nil {
		return next
	}

	return func(request *http.Request) (*http.Response, error) {
		start := l.sent()
		response, err := next(request)
		l.record(service, request.Method, safePath(request), response, start, l.now().Sub(start))

		return response, err
	}
}

// sent counts a request out, and is when it went.
func (l *RequestLog) sent() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.inFlight++

	return l.now()
}

// record counts a request answered, writes its outline line to the log
// while it records, and says it to the echo: the start time, the service, the
// method, the safe path, the status, and how long the request took.
func (l *RequestLog) record(
	service, method, safePath string, response *http.Response, start time.Time, elapsed time.Duration,
) {
	outline := fmt.Sprintf("%s %-5s %-4s %s %s %s\n",
		start.UTC().Format(time.RFC3339), service, method,
		safePath, status(response), elapsed.Round(time.Millisecond))

	l.mu.Lock()
	defer l.mu.Unlock()

	l.inFlight--

	_, _ = io.WriteString(l.echo, outline)

	if l.stage != logRecording {
		return
	}

	_, err := io.WriteString(l.out, outline)
	if err != nil && l.writeErr == nil {
		l.writeErr = err
	}
}

// status is the response's code, or a dash when the request never got one.
func status(response *http.Response) string {
	if response == nil {
		return "—"
	}

	return strconv.Itoa(response.StatusCode)
}

// loggedPath is a request path safe to record. A Slack webhook's path is its
// credential, so everything past the prefix is dropped; every other path is a
// route, not a secret, and is kept.
func loggedPath(path string) string {
	if strings.HasPrefix(path, webhookPrefix) {
		return webhookPrefix + redacted
	}

	return path
}
