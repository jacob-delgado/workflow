// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package httpx is the HTTP transport the Jira, forge and Slack clients share:
// the one-method Doer seam they accept, and a client that refuses redirects so a
// credential never follows one.
package httpx

import (
	"errors"
	"net/http"
	"time"
)

// ErrRedirected reports a redirect this client declined to follow.
var ErrRedirected = errors.New("refused to follow a redirect")

// ErrRateLimited reports a server that answered 429 Too Many Requests. It is its
// own error so a caller is told to wait rather than that its credential was
// refused, which a shared 403/429 branch would say.
var ErrRateLimited = errors.New("rate limited; wait and try again")

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
