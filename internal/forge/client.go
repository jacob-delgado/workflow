// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

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
	// ErrUnexpectedStatus reports any other status.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the forge")
	// ErrRedirected reports a redirect this client declined to follow.
	ErrRedirected = errors.New("refused to follow a redirect")
)

// Doer sends one HTTP request. *http.Client's Do method satisfies it.
type Doer func(*http.Request) (*http.Response, error)

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

// HTTPClient is the transport a credential may travel over. It refuses
// redirects: Go forwards the Authorization header to any target sharing the
// origin's host OR a subdomain of it, ignoring the port and the scheme.
func HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirected
		},
	}
}

// Whoami reports which account the credential belongs to.
func (c Client) Whoami(ctx context.Context) (Identity, error) {
	if c.token == "" {
		return Identity{}, ErrNoToken
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+userPath, nil)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return Identity{}, fmt.Errorf("%w: the API address could not be used", ErrUnreachable)
	}

	// GitLab accepts a bearer token for a personal access token, so both forges
	// take the same header. Its own PRIVATE-TOKEN header would be worse: Go
	// forwards a custom header to EVERY redirect target, with none of the
	// same-host check it applies to Authorization.
	request.Header.Set("Authorization", "Bearer "+c.token.Secret())
	request.Header.Set("Accept", jsonMediaType)
	request.Header.Set("User-Agent", userAgent)

	return c.send(request)
}

// send performs the request and decodes the answer.
func (c Client) send(request *http.Request) (Identity, error) {
	response, err := c.do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("%w at %s: %w", ErrUnreachable, c.base, err)
	}
	defer func() { _ = response.Body.Close() }()

	err = statusError(response.StatusCode)
	if err != nil {
		return Identity{}, err
	}

	err = mustBeJSON(response.Header.Get("Content-Type"))
	if err != nil {
		return Identity{}, err
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return Identity{}, fmt.Errorf("reading the answer from %s: %w", c.base, err)
	}

	var identity Identity

	// Every string in the answer is about to be printed, and a forge — or
	// anything answering in its place — chose it.
	err = json.Unmarshal(sanitize.JSON(body), &identity)
	if err != nil {
		return Identity{}, fmt.Errorf("reading the answer from %s: %w", c.base, err)
	}

	return identity, nil
}

// mustBeJSON rejects an answer that is not JSON.
//
// A base URL aimed at the web host rather than the API answers 200 with a login
// page, and decoding that produces "invalid character '<'" — which tells nobody
// what went wrong. Checking first also means no unbounded HTML body is ever
// streamed into the decoder.
func mustBeJSON(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != jsonMediaType {
		return fmt.Errorf("%w but %q — does the API address point at the web host?",
			ErrNotJSON, contentType)
	}

	return nil
}

// statusError translates a response status into something a person can act on.
// The body is never read: GitHub answers a missing User-Agent and a rate limit
// with the same 403, and neither body says which.
func statusError(status int) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden, http.StatusTooManyRequests:
		return ErrRefused
	case http.StatusNotFound:
		return ErrNoAPI
	default:
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
	}
}
