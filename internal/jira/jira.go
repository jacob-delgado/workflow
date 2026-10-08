// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package jira talks to a Jira Data Center instance over its REST v2 API.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// myselfPath answers "does this credential work?". Jira documents it as 200 or
// 401 and says it cannot be read anonymously, which is what makes it a probe;
// /rest/api/2/serverInfo, the obvious alternative, answers 200 to strangers.
const myselfPath = "/rest/api/2/myself"

// jsonMediaType is what Jira is asked to answer in, and what an answer must be
// before it is decoded.
const jsonMediaType = "application/json"

// bodyLimit bounds how much of an answer is read. An issue with a thousand
// comments is a few megabytes; an answer past this is not one worth decoding.
const bodyLimit = 16 << 20

// Errors this package returns. Callers distinguish them with errors.Is. A
// refused redirect and a rate limit come back as httpx's ErrRedirected and
// ErrRateLimited, which every client shares.
var (
	// ErrNoCredential reports that Jira has no token to send: none is
	// configured, or the configured source gave none.
	ErrNoCredential = errors.New("no Jira token to send")
	// ErrUnauthorized reports a credential the instance did not accept.
	ErrUnauthorized = errors.New("the credential was not accepted")
	// ErrForbidden reports a 403: a credential the instance refused to consider,
	// or would not let do this. A 403 Jira explained carries it alongside
	// ErrRejected, so a caller can tell a refused token from any other rejection.
	ErrForbidden = errors.New("the credential was refused")
	// ErrNoAPI reports a URL with no Jira REST API behind it.
	ErrNoAPI = errors.New("no Jira REST API answered at that address")
	// ErrNotFound reports a 404: the addressed resource, most often an issue, does
	// not exist. A 404 answer carries it alongside its more specific error, so a
	// caller can tell a missing issue from any other rejection.
	ErrNotFound = errors.New("the resource was not found")
	// ErrRejected reports a request Jira refused and explained; the explanation
	// follows it in the message.
	ErrRejected = errors.New("jira rejected the request")
	// ErrUnexpectedStatus reports any other status.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrNotJSON reports an answer that was not JSON, which usually means the
	// base URL reaches a sign-in page or a proxy rather than Jira's REST API.
	ErrNotJSON = errors.New("the answer was not JSON")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the server")
)

// Doer is the HTTP seam this client accepts; see httpx.Doer.
type Doer = httpx.Doer

// User is who a credential authenticates as.
type User struct {
	// DisplayName is the human name, which an instance may be configured to
	// withhold; Name is the login, which it does not.
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
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

// Myself reports who the configured credential authenticates as.
func (c Client) Myself(ctx context.Context) (User, error) {
	request, err := c.newRequest(ctx, http.MethodGet, myselfPath, nil)

	return decode[User](c, request, err)
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
		return nil, config.ErrInvalidBaseURL
	}

	err = config.CheckBaseURL(c.settings.BaseURL)
	if err != nil {
		return nil, err
	}

	// Without this Jira answers errors in XML rather than JSON.
	request.Header.Set("Accept", jsonMediaType)
	authenticate(request, c.settings)

	// After the token, so a proxy in front of Jira can be given the header it
	// wants — and so an intentional override, such as a different Authorization,
	// is the one that survives.
	for name, value := range c.settings.Headers {
		request.Header.Set(name, value.Reveal())
	}

	return request, nil
}

// newJSONRequest builds an authenticated request that sends body as JSON, the
// shape every write to Jira takes.
func (c Client) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	// Trade-off TRADE-13: every body is one of this package's request types,
	// which always encode.
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the request: %w", err)
	}

	request, err := c.newRequest(ctx, method, path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", jsonMediaType)

	return request, nil
}

// authenticators maps each usable mode to the way it signs a request.
//
// A map built here rather than a switch, for two reasons: gochecknoglobals
// forbids the package-level table, and a switch over every AuthMode leaves a
// final arm that can never be false once the others cover the domain. The
// absence of AuthNone from this map is what makes it an error.
func authenticators() map[config.AuthMode]func(*http.Request, config.Jira) {
	//nolint:exhaustive // AuthNone is absent by design; its lookup miss is what makes it ErrNoCredential.
	return map[config.AuthMode]func(*http.Request, config.Jira){
		config.AuthBearer: func(request *http.Request, settings config.Jira) {
			request.Header.Set("Authorization", "Bearer "+settings.Token.Reveal())
		},
		config.AuthBasic: func(request *http.Request, settings config.Jira) {
			request.SetBasicAuth(settings.User, settings.Token.Reveal())
		},
	}
}

// exchange sends a request and returns the body of the answer, only if Jira
// accepted it and only once nothing in it can drive a terminal: every string in
// it is text someone else chose, about to be drawn on this one.
func (c Client) exchange(request *http.Request) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, httpx.Unreachable(ErrUnreachable, c.settings.BaseURL, err)
	}
	defer func() { _ = response.Body.Close() }()

	err = c.answerError(response, request.URL)
	if err != nil {
		return nil, err
	}

	err = mustBeJSON(response)
	if err != nil {
		return nil, err
	}

	body, err := httpx.Read(response.Body, bodyLimit)
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.settings.BaseURL, err)
	}

	return sanitize.JSON(body), nil
}

// mustBeJSON rejects an answer with a body that is not JSON. An answer with no
// body, as a 204 or an empty 201 is, has nothing to be anything.
//
// A base URL that reaches a sign-in page or a proxy rather than Jira answers
// 200 with HTML, and decoding that produces "invalid character '<'" — which
// tells nobody what went wrong.
func mustBeJSON(response *http.Response) error {
	if response.StatusCode == http.StatusNoContent || response.ContentLength == 0 {
		return nil
	}

	contentType := response.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != jsonMediaType {
		return fmt.Errorf("%w but %q — does jira.base_url reach Jira itself rather than a sign-in page?",
			ErrNotJSON, contentType)
	}

	return nil
}

// decode sends a built request and unmarshals the answer into T — the
// exchange-then-unmarshal every read method shares. buildErr is newRequest's
// error, threaded in so a caller need not check it separately; the caller
// flattens T (often a wire shape) into the domain type it returns.
func decode[T any](client Client, request *http.Request, buildErr error) (T, error) {
	var answer T

	if buildErr != nil {
		return answer, buildErr
	}

	body, err := client.exchange(request)
	if err != nil {
		return answer, err
	}

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return answer, fmt.Errorf("reading the answer from %s: %w", client.settings.BaseURL, err)
	}

	return answer, nil
}

// statusError translates a response status into something a person can act on.
// The body is never read: a failed Basic login is answered with an HTML login
// page even when JSON was asked for, so it explains nothing.
func statusError(status int, requested *url.URL) error {
	switch status {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		// The address is the useful part here: a missing context path, such as
		// the /jira that many on-prem instances live under, looks exactly like
		// this. Redacted() masks any userinfo that slipped past CheckBaseURL.
		return fmt.Errorf("%w: %s", ErrNoAPI, requested.Redacted())
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}
