// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package httpx is the HTTP transport the Jira, forge and messaging clients share:
// the one-method Doer seam they accept, and a client that refuses redirects so a
// credential never follows one.
package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrRedirected reports a redirect this client declined to follow.
var ErrRedirected = errors.New("refused to follow a redirect")

// Cause strips net/http's *url.Error down to what actually went wrong. Its
// message quotes the whole request URL first — for a search, a long line of
// encoded query a pane would clip — so the clients show the inner error rather
// than let the address bury it, and read the same way while doing so.
func Cause(err error) error {
	transportErr, ok := errors.AsType[*url.Error](err)
	if ok {
		return transportErr.Err
	}

	return err
}

// Unreachable turns a transport error into a message that reads the way what
// happened warrants. A refused redirect means the server DID answer — with a
// redirect this client declines to follow — so it is not "could not reach"; it
// is reported as ErrRedirected. Anything else wraps the caller's own unreachable
// sentinel (whose noun names the service). base is named when non-empty; a
// client whose address is itself a credential — a Slack webhook — passes "".
func Unreachable(unreachable error, base string, err error) error {
	cause := Cause(err)

	atBase := ""
	if base != "" {
		atBase = " at " + base
	}

	if errors.Is(cause, ErrRedirected) {
		return fmt.Errorf("the server%s redirected the request: %w", atBase, ErrRedirected)
	}

	return fmt.Errorf("%w%s: %w", unreachable, atBase, cause)
}

// ErrRateLimited reports a server that answered 429 Too Many Requests. It is its
// own error so a caller is told to wait rather than that its credential was
// refused, which a shared 403/429 branch would say.
var ErrRateLimited = errors.New("rate limited; wait and try again")

// RateLimited reports a 429, naming how long to wait when the server said so in
// a Retry-After header. The header's other, HTTP-date form — which these APIs do
// not use for a 429 — and a missing or unreadable value fall back to the bare
// ErrRateLimited, so a caller always gets an error that errors.Is matches.
func RateLimited(header http.Header) error {
	wait, ok := retryAfter(header.Get("Retry-After"))
	if !ok {
		return ErrRateLimited
	}

	return fmt.Errorf("%w (in %s)", ErrRateLimited, wait)
}

// retryAfter reads the delta-seconds form of a Retry-After header.
func retryAfter(value string) (time.Duration, bool) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0, false
	}

	return time.Duration(seconds) * time.Second, true
}

// Doer sends one HTTP request. *http.Client's Do method satisfies it.
//
// A function type rather than an interface because it is a single method, which
// is the seam shape this project prefers; it exists so a caller can supply the
// redirect-refusing, deadline-bearing client below rather than the default one.
type Doer func(*http.Request) (*http.Response, error)

// Client is the transport a credential may travel over.
//
// It refuses redirects, and that is the point rather than a convenience: Go's
// default client forwards the Authorization header to any redirect target
// sharing a HOSTNAME, ignoring both the port and the scheme. A service behind a
// misconfigured proxy that redirects HTTPS to HTTP on the same host would hand
// the token to the plaintext hop with nothing in the code looking wrong.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirected
		},
	}
}
