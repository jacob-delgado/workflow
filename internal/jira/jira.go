// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package jira talks to a Jira Data Center instance over its REST v2 API.
//
// Only what doctor needs so far: asking the instance who the configured
// credential authenticates as, which is the one question that cannot be
// answered without leaving the machine.
package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// myselfPath answers "does this credential work?". Jira documents it as 200 or
// 401 and says it cannot be read anonymously, which is what makes it a probe;
// /rest/api/2/serverInfo, the obvious alternative, answers 200 to strangers.
const myselfPath = "/rest/api/2/myself"

// Errors this package returns. Callers distinguish them with errors.Is.
var (
	// ErrNoCredential reports a configuration with no Jira token at all.
	ErrNoCredential = errors.New("no jira.token is configured")
	// ErrInvalidBaseURL reports a jira.base_url that is not an absolute URL.
	ErrInvalidBaseURL = errors.New("jira.base_url is not an absolute http or https URL")
	// ErrCredentialInBaseURL reports userinfo embedded in jira.base_url.
	ErrCredentialInBaseURL = errors.New("jira.base_url carries a username and password")
	// ErrUnauthorized reports a credential the instance did not accept.
	ErrUnauthorized = errors.New("the credential was not accepted")
	// ErrForbidden reports a credential the instance refused to consider.
	ErrForbidden = errors.New("the credential was refused")
	// ErrNoAPI reports a URL with no Jira REST API behind it.
	ErrNoAPI = errors.New("no Jira REST API answered at that address")
	// ErrRejected reports a request Jira refused and explained; the explanation
	// follows it in the message.
	ErrRejected = errors.New("jira rejected the request")
	// ErrUnexpectedStatus reports any other status.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the server")
	// ErrRedirected reports a redirect this client declined to follow.
	ErrRedirected = errors.New("refused to follow a redirect")
)

// Doer sends one HTTP request. *http.Client's Do method satisfies it.
//
// A function type rather than an interface because it is a single method, which
// is the seam shape this project prefers; it exists so a caller can supply the
// redirect-refusing, deadline-bearing client below rather than the default one.
type Doer func(*http.Request) (*http.Response, error)

// User is who a credential authenticates as.
//
//nolint:tagliatelle // these are Jira's field names on the wire, not ours to pick
type User struct {
	// DisplayName is the human name, which an instance may be configured to
	// withhold; Name is the login, which it does not.
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	Active      bool   `json:"active"`
}

// Client talks to one Jira instance.
type Client struct {
	do       Doer
	settings config.Jira
}

// New builds a client for the instance settings describe.
func New(do Doer, settings config.Jira) Client {
	// A trailing slash would produce a doubled slash in the path, which a real
	// Jira answers with 404 rather than ignoring.
	settings.BaseURL = strings.TrimRight(settings.BaseURL, "/")

	return Client{do: do, settings: settings}
}

// HTTPClient is the transport a credential may travel over.
//
// It refuses redirects, and that is the point rather than a convenience: Go's
// default client forwards the Authorization header to any redirect target
// sharing a HOSTNAME, ignoring both the port and the scheme. An on-prem Jira
// behind a misconfigured proxy that redirects HTTPS to HTTP on the same host
// would hand the token to the plaintext hop with nothing in the code looking
// wrong.
func HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirected
		},
	}
}

// Myself reports who the configured credential authenticates as.
func (c Client) Myself(ctx context.Context) (User, error) {
	request, err := c.newRequest(ctx, http.MethodGet, myselfPath, nil)
	if err != nil {
		return User{}, err
	}

	response, err := c.do(request)
	if err != nil {
		return User{}, fmt.Errorf("%w at %s: %w", ErrUnreachable, c.settings.BaseURL, err)
	}
	defer func() { _ = response.Body.Close() }()

	err = statusError(response.StatusCode, request.URL)
	if err != nil {
		return User{}, err
	}

	var user User

	err = json.NewDecoder(response.Body).Decode(&user)
	if err != nil {
		return User{}, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return user, nil
}

// newRequest builds an authenticated request for a path under the base URL,
// refusing a base URL that cannot carry a credential safely.
func (c Client) newRequest(ctx context.Context, method, pathAndQuery string, body io.Reader) (*http.Request, error) {
	authenticate, ok := authenticators()[c.settings.AuthMode()]
	if !ok {
		return nil, ErrNoCredential
	}

	request, err := http.NewRequestWithContext(ctx, method, c.settings.BaseURL+pathAndQuery, body)
	if err != nil {
		// Deliberately unwrapped: the parse error quotes the whole URL, so a
		// base_url carrying userinfo would put the password into this message.
		return nil, ErrInvalidBaseURL
	}

	err = usable(request.URL)
	if err != nil {
		return nil, err
	}

	// Without this Jira answers errors in XML rather than JSON.
	request.Header.Set("Accept", "application/json")
	authenticate(request, c.settings)

	return request, nil
}

// usable reports whether a parsed base URL can carry a credential.
func usable(address *url.URL) error {
	// net/http turns userinfo into an Authorization header of its own, which
	// would compete with the configured token — and the winner is not the one
	// the reader of the configuration file expects.
	if address.User != nil {
		return ErrCredentialInBaseURL
	}

	if address.Host == "" || (address.Scheme != "https" && address.Scheme != "http") {
		return ErrInvalidBaseURL
	}

	return nil
}

// authenticators maps each usable mode to the way it signs a request.
//
// A map built here rather than a switch, for two reasons: gochecknoglobals
// forbids the package-level table, and a switch over every AuthMode leaves a
// final arm that can never be false once the others cover the domain. The
// absence of AuthNone from this map is what makes it an error.
func authenticators() map[config.AuthMode]func(*http.Request, config.Jira) {
	return map[config.AuthMode]func(*http.Request, config.Jira){
		config.AuthBearer: func(request *http.Request, settings config.Jira) {
			request.Header.Set("Authorization", "Bearer "+settings.Token)
		},
		config.AuthBasic: func(request *http.Request, settings config.Jira) {
			request.SetBasicAuth(settings.User, settings.Token)
		},
	}
}

// exchange sends a request and hands back the answer only if Jira accepted it.
// The caller closes the body of an answer it receives.
func (c Client) exchange(request *http.Request) (*http.Response, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("%w at %s: %w", ErrUnreachable, c.settings.BaseURL, cause(err))
	}

	err = c.answerError(response, request.URL)
	if err != nil {
		_ = response.Body.Close()

		return nil, err
	}

	return response, nil
}

// statusError translates a response status into something a person can act on.
// The body is never read: a failed Basic login is answered with an HTML login
// page even when JSON was asked for, so it explains nothing.
func statusError(status int, requested *url.URL) error {
	switch status {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		// The address is the useful part here: a missing context path, such as
		// the /jira that many on-prem instances live under, looks exactly like
		// this. Redacted() masks any userinfo that slipped past usable.
		return fmt.Errorf("%w: %s", ErrNoAPI, requested.Redacted())
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}
