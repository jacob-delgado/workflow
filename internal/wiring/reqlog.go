// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// webhookPrefix is the start of a Slack incoming-webhook path. The rest of that
// path is the credential itself, so it is the one path that must never be logged
// in full.
const webhookPrefix = "/services/"

// RequestLog records the outline of each HTTP request — its service, method,
// path, status and duration — for a bug report. It records nothing else: never a
// header, a body, a query string or a host, so no credential reaches the file.
type RequestLog struct {
	mu  sync.Mutex
	out io.Writer
	now func() time.Time
}

// NewRequestLog writes outlines to out, timestamped by now. A nil now means
// time.Now.
func NewRequestLog(out io.Writer, now func() time.Time) *RequestLog {
	if now == nil {
		now = time.Now
	}

	return &RequestLog{out: out, now: now}
}

// Wrap decorates a Doer so each request it makes is recorded under service. A
// nil log wraps to the same Doer, so a caller need not branch on whether logging
// is on.
func (l *RequestLog) Wrap(service string, next func(*http.Request) (*http.Response, error),
) func(*http.Request) (*http.Response, error) {
	if l == nil {
		return next
	}

	return func(request *http.Request) (*http.Response, error) {
		start := l.now()
		response, err := next(request)
		l.record(service, request, response, start, l.now().Sub(start))

		return response, err
	}
}

// record writes one outline line: the start time, the service, the method, the
// safe path, the status, and how long the request took.
func (l *RequestLog) record(
	service string, request *http.Request, response *http.Response, start time.Time, elapsed time.Duration,
) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.out, "%s %-5s %-4s %s %s %s\n",
		start.UTC().Format(time.RFC3339), service, request.Method,
		loggedPath(request.URL.Path), status(response), elapsed.Round(time.Millisecond))
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
		return webhookPrefix + "[redacted]"
	}

	return path
}
