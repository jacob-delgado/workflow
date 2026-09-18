// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package slack posts to Slack, through a bot token or an incoming webhook, and
// asks whether a bot token works. A webhook has no equivalent question — see
// ErrWebhookUncheckable.
package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// APIBase is where the Slack Web API lives. Unlike Jira, it is the same for
// everyone, so it is a constant rather than a setting — but it stays a parameter
// of New so a test can point the client somewhere it controls.
const APIBase = "https://slack.com/api"

// bodyLimit bounds how much of an answer is read.
const bodyLimit = 1 << 20

// authTestPath answers "does this token work?" without posting anything.
const authTestPath = "/auth.test"

// Errors this package returns. Callers distinguish them with errors.Is.
var (
	// ErrNoCredential reports a configuration with neither transport set.
	ErrNoCredential = errors.New("no slack.token or slack.webhook_url is configured")
	// ErrWebhookUncheckable reports that a webhook cannot be verified.
	ErrWebhookUncheckable = errors.New("an incoming webhook cannot be checked without posting with it")
	// ErrRejected reports a token Slack would not accept.
	ErrRejected = errors.New("the credential was not accepted")
	// ErrPostRefused reports a post Slack would not deliver, for a reason that is
	// not about the credential.
	ErrPostRefused = errors.New("the post was refused")
	// ErrUnexpectedStatus reports a response status the API does not document.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the Slack API")
	// ErrRedirected reports a redirect this client declined to follow.
	ErrRedirected = httpx.ErrRedirected
	// ErrRateLimited reports a 429 from Slack's API.
	ErrRateLimited = httpx.ErrRateLimited
)

// Doer is the HTTP seam this client accepts; see httpx.Doer.
type Doer = httpx.Doer

// Identity is the workspace and bot user a token belongs to.
type Identity struct {
	// OK is Slack's verdict. It is the field that matters: the Web API answers
	// a dead token with HTTP 200 and ok:false, so the status line lies.
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	User  string `json:"user"`
	Team  string `json:"team"`
}

// Client talks to the Slack Web API.
type Client struct {
	do    Doer
	base  string
	creds config.Slack
}

// New builds a client. Pass APIBase unless you are a test.
func New(do Doer, base string, creds config.Slack) Client {
	return Client{do: do, base: strings.TrimRight(base, "/"), creds: creds}
}

// HTTPClient is the redirect-refusing transport a Slack credential travels
// over; see httpx.Client for why refusing matters.
func HTTPClient(timeout time.Duration) *http.Client {
	return httpx.Client(timeout)
}

// AuthTest reports which workspace and user the configured token belongs to.
func (c Client) AuthTest(ctx context.Context) (Identity, error) {
	err := c.checkable()
	if err != nil {
		return Identity{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+authTestPath, nil)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return Identity{}, fmt.Errorf("%w: building the request", ErrUnreachable)
	}

	// Slack also accepts the token as a query parameter. It travels in the
	// header instead, so it stays out of proxy logs and browser history.
	request.Header.Set("Authorization", "Bearer "+c.creds.Token.Reveal())
	request.Header.Set("Accept", "application/json")

	return c.send(request)
}

// checkable reports whether this configuration has something worth asking about.
func (c Client) checkable() error {
	switch c.creds.Mode() {
	case config.SlackBot:
		return nil
	case config.SlackWebhook:
		// The only way to learn whether a webhook works is to post with it, and
		// that would put a test message in somebody's channel.
		return ErrWebhookUncheckable
	case config.SlackNone:
		return ErrNoCredential
	default:
		return ErrNoCredential
	}
}

// send performs the request and reads Slack's verdict out of the body.
func (c Client) send(request *http.Request) (Identity, error) {
	response, err := c.do(request)
	if err != nil {
		return Identity{}, httpx.Unreachable(ErrUnreachable, "", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusTooManyRequests {
		return Identity{}, httpx.RateLimited(response.Header)
	}

	if response.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, bodyLimit))
	if err != nil {
		return Identity{}, fmt.Errorf("reading the answer from Slack: %w", err)
	}

	var identity Identity

	// Workspace and user names, and Slack's own error, are about to be printed.
	err = json.Unmarshal(sanitize.JSON(body), &identity)
	if err != nil {
		return Identity{}, fmt.Errorf("reading the answer from Slack: %w", err)
	}

	// The status was 200 and the answer is still no. This is Slack's convention,
	// and reading the status alone would call a dead token healthy.
	if !identity.OK {
		return Identity{}, fmt.Errorf("%w: %s", ErrRejected, identity.Error)
	}

	return identity, nil
}
