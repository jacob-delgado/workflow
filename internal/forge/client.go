// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// userPath answers "who does this credential belong to?" on both forges. It
// needs no scope on GitHub, which is what makes a 404 here mean a wrong address
// rather than an under-scoped token.
const userPath = "/user"

// userAgent identifies this program. It must never be empty: Go REMOVES the
// header when asked to set it to "", and GitHub answers 403 to a request with
// no User-Agent — a status that would otherwise read as a refused credential.
const userAgent = "workflow (+https://github.com/jacob-delgado/workflow)"

// bodyLimit bounds how much of an answer is read.
const bodyLimit = 16 << 20

// jsonMediaType is what an API answer must be before it is decoded.
const jsonMediaType = "application/json"

// Errors the client returns. Callers distinguish them with errors.Is.
var (
	// ErrUnauthorized reports a credential the forge did not accept.
	ErrUnauthorized = errors.New("the credential was not accepted")
	// ErrRefused reports a request the forge would not serve. On GitHub this
	// covers rate limiting and a temporary lockout as well as a bad request, so
	// it deliberately does not claim the credential is wrong.
	ErrRefused = errors.New("the forge refused the request, which may be rate limiting rather than the credential")
	// ErrNoAPI reports an address with no forge API behind it.
	ErrNoAPI = errors.New("no forge API answered at that address")
	// ErrNotJSON reports an answer that was not JSON, which usually means the
	// base URL points at the web host rather than the API.
	ErrNotJSON = errors.New("the answer was not JSON")
	// ErrRejected reports a request the forge turned down and explained; the
	// explanation follows it in the message.
	ErrRejected = errors.New("the forge rejected the request")
	// ErrNoRepository reports a repository the forge will not show this token:
	// both forges answer 404 rather than 403, so as not to confirm it exists.
	ErrNoRepository = errors.New("the repository was not found, or the token cannot see it")
	// ErrUnexpectedStatus reports any other status.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the forge")
	// ErrRedirected reports a redirect this client declined to follow.
	ErrRedirected = httpx.ErrRedirected
	// ErrRateLimited reports a 429 from the forge, told apart from a refusal.
	ErrRateLimited = httpx.ErrRateLimited
)

// Doer is the HTTP seam this client accepts; see httpx.Doer.
type Doer = httpx.Doer

// Identity is who a credential belongs to. The two forges name the same thing
// differently, which is the only difference this package has to care about.
type Identity struct {
	Login    string `json:"login"`
	Username string `json:"username"`
}

// Name is the account name, whichever field the forge used.
func (i Identity) Name() string {
	if i.Login != "" {
		return i.Login
	}

	return i.Username
}

// Client talks to one forge's REST API.
type Client struct {
	do    Doer
	base  string
	token Token
}

// New builds a client. Pass the base URL from Repo.APIBase.
func New(do Doer, base string, token Token) Client {
	return Client{do: do, base: strings.TrimRight(base, "/"), token: token}
}

// HTTPClient is the redirect-refusing transport a forge credential travels
// over; see httpx.Client for why refusing matters.
func HTTPClient(timeout time.Duration) *http.Client {
	return httpx.Client(timeout)
}

// Whoami reports which account the credential belongs to.
func (c Client) Whoami(ctx context.Context) (Identity, error) {
	return call[Identity](ctx, c, http.MethodGet, userPath, nil)
}

// call sends one request to the forge and decodes the answer into T.
func call[T any](ctx context.Context, client Client, method, path string, payload any) (T, error) {
	var answer T

	if client.token == "" {
		return answer, ErrNoToken
	}

	request, err := client.newRequest(ctx, method, path, payload)
	if err != nil {
		return answer, err
	}

	body, err := client.exchange(request)
	if err != nil {
		return answer, err
	}

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return answer, fmt.Errorf("reading the answer from %s: %w", client.base, err)
	}

	return answer, nil
}

// newRequest builds an authenticated request, with payload encoded as its JSON
// body when there is one.
func (c Client) newRequest(ctx context.Context, method, path string, payload any) (*http.Request, error) {
	var body io.Reader

	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding the request: %w", err)
		}

		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return nil, fmt.Errorf("%w: the API address could not be used", ErrUnreachable)
	}

	// GitLab accepts a bearer token for a personal access token, so both forges
	// take the same header. Its own PRIVATE-TOKEN header would be worse: Go
	// forwards a custom header to EVERY redirect target, with none of the
	// same-host check it applies to Authorization.
	request.Header.Set("Authorization", "Bearer "+c.token.Secret())
	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("User-Agent", userAgent)

	if payload != nil {
		request.Header.Set("Content-Type", jsonMediaType)
	}

	return request, nil
}

// exchange performs the request and returns the answer's body, only once the
// forge accepted the request, answered in JSON, and nothing in the answer can
// drive a terminal.
func (c Client) exchange(request *http.Request) ([]byte, error) {
	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("%w at %s: %w", ErrUnreachable, c.base, httpx.Cause(err))
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusTooManyRequests {
		return nil, httpx.RateLimited(response.Header)
	}

	err = statusError(response.StatusCode)
	if err != nil {
		return nil, explained(err, response.Body)
	}

	err = mustBeJSON(response.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("reading the answer from %s: %w", c.base, err)
	}

	// Every string in the answer is about to be shown, and a forge — or
	// anything answering in its place — chose it.
	return sanitize.JSON(body), nil
}

// mustBeJSON rejects an answer that is not JSON.
//
// A base URL aimed at the web host rather than the API answers 200 with a login
// page, and decoding that produces "invalid character '<'" — which tells nobody
// what went wrong.
func mustBeJSON(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != jsonMediaType {
		return fmt.Errorf("%w but %q — does the API address point at the web host?",
			ErrNotJSON, contentType)
	}

	return nil
}

// statusError translates a response status into something a person can act on.
// Its body is read only for a request the forge understood and turned down —
// see explained — because GitHub answers a missing User-Agent and a rate limit
// with the same 403, and neither body says which.
func statusError(status int) error {
	switch status {
	case http.StatusOK, http.StatusCreated:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrRefused
	case http.StatusNotFound:
		return ErrNoAPI
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}
