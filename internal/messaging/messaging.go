// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package messaging posts team announcements — to Slack with a rotating user
// token or through an incoming webhook, and to Teams, Discord or a plain webhook
// through theirs — and asks whether a Slack user token works. A webhook has no equivalent question —
// see ErrWebhookUncheckable.
package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

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

// Errors this package returns. Callers distinguish them with errors.Is. A
// refused redirect and a rate limit come back as httpx's ErrRedirected and
// ErrRateLimited, which every client shares.
var (
	// ErrNoCredential reports that messaging has no credential to post with:
	// neither transport is set, or the user token has no source to give one.
	ErrNoCredential = errors.New("messaging has no credential")
	// ErrWebhookUncheckable reports that a webhook cannot be verified.
	ErrWebhookUncheckable = errors.New("an incoming webhook cannot be checked without posting with it")
	// ErrRejected reports a credential the service would not accept: Slack's no
	// to a token, or any service's 4xx answer to a post.
	ErrRejected = errors.New("the credential was not accepted")
	// ErrTokenExpired reports a user token Slack called expired after the
	// source was asked for a newer one, which it answers as ErrRejected too.
	ErrTokenExpired = errors.New("the Slack user token has expired")
	// ErrPostRefused reports a message Slack would not deliver, for a reason
	// that is not about the credential. Its words name no verb, because an
	// announcement and the Summary both post through here.
	ErrPostRefused = errors.New("the message was refused")
	// ErrUnexpectedStatus reports a response status the API does not document.
	ErrUnexpectedStatus = errors.New("unexpected response status")
	// ErrUnreachable reports a request that never got an answer.
	ErrUnreachable = errors.New("could not reach the messaging service")
	// ErrNoWorkspace reports an auth.test answer naming no Slack workspace, or
	// one not shaped like a workspace's ID.
	ErrNoWorkspace = errors.New("auth.test named no Slack workspace for the token")
)

// Doer is the HTTP seam this client accepts; see httpx.Doer.
type Doer = httpx.Doer

// Identity is the workspace and user a token belongs to.
type Identity struct {
	// OK is Slack's verdict. It is the field that matters: the Web API answers
	// a dead token with HTTP 200 and ok:false, so the status line lies.
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	User  string `json:"user"`
	Team  string `json:"team"`
	// TeamID is the workspace's ID, which, unlike its name, never changes.
	TeamID string `json:"team_id"`
	// Granted is the scopes Slack lists the token as granted.
	Granted Grant `json:"-"`
}

// Workspace is the ID of the Slack workspace the identity is in: T and
// capitals and digits, or E for an Enterprise Grid organization's token.
func (i Identity) Workspace() (string, error) {
	if !slackID(i.TeamID, "TE") {
		return "", ErrNoWorkspace
	}

	return i.TeamID, nil
}

// scopesHeader is where auth.test lists the scopes a token was granted.
const scopesHeader = "X-Oauth-Scopes"

// Grant is the scopes Slack granted a token, as auth.test lists them.
type Grant struct {
	scopes []string
	listed bool
}

// Lacks reports a scope Slack listed the token without. A grant Slack did not
// list lacks none: only a read's own refusal can tell then.
func (g Grant) Lacks(scope string) bool {
	return g.listed && !slices.Contains(g.scopes, scope)
}

// grantOf is the grant a response's header lists.
func grantOf(header http.Header) Grant {
	listed := header.Get(scopesHeader)
	if listed == "" {
		return Grant{scopes: nil, listed: false}
	}

	scopes := strings.Split(listed, ",")
	for index, scope := range scopes {
		scopes[index] = strings.TrimSpace(scope)
	}

	return Grant{scopes: scopes, listed: true}
}

// TokenSource hands out the Slack user token to send: one other than expired,
// the token Slack just called expired, or any when expired is empty.
type TokenSource func(ctx context.Context, expired config.Secret) (config.Secret, error)

// Client talks to a messaging service — Slack's Web API, or an incoming webhook
// for Slack, Teams, Discord or a plain endpoint.
type Client struct {
	do    Doer
	base  string
	creds config.Messaging
	token TokenSource
}

// New builds a client. Pass APIBase unless you are a test.
func New(do Doer, base string, creds config.Messaging) Client {
	return Client{do: do, base: strings.TrimRight(base, "/"), creds: creds, token: nil}
}

// WithToken is the client asking source for the Slack user token on every call,
// since a rotating token can expire between two of them.
func (c Client) WithToken(source TokenSource) Client {
	c.token = source

	return c
}

// AuthTest reports which workspace and user the configured token belongs to.
func (c Client) AuthTest(ctx context.Context) (Identity, error) {
	err := c.checkable()
	if err != nil {
		return Identity{}, err
	}

	var identity Identity

	err = c.withFreshToken(ctx, func(token config.Secret) error {
		identity, err = c.authTest(ctx, token)

		return err
	})

	return identity, err
}

// authTest asks auth.test about token.
func (c Client) authTest(ctx context.Context, token config.Secret) (Identity, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+authTestPath, nil)
	if err != nil {
		// Unwrapped: the parse error quotes the whole URL.
		return Identity{}, fmt.Errorf("%w: building the request", ErrUnreachable)
	}

	// Slack also accepts the token as a query parameter. It travels in the
	// header instead, so it stays out of proxy logs and browser history.
	request.Header.Set("Authorization", "Bearer "+token.Reveal())
	request.Header.Set("Accept", "application/json")

	return c.send(request)
}

// withFreshToken makes call with the user token the source gives, and once
// more with a newer one when Slack calls that token expired.
func (c Client) withFreshToken(ctx context.Context, call func(config.Secret) error) error {
	if c.token == nil {
		return ErrNoCredential
	}

	token, err := c.token(ctx, "")
	if err != nil {
		return err
	}

	err = call(token)
	if !errors.Is(err, ErrTokenExpired) {
		return err
	}

	token, err = c.token(ctx, token)
	if err != nil {
		return err
	}

	return call(token)
}

// checkable reports whether this configuration has something worth asking about.
// A map, not a switch, so there is no last-case arm gobco can never see;
// exhaustive keeps it complete.
func (c Client) checkable() error {
	return map[config.MessagingMode]error{
		config.MessagingUser: nil,
		// The only way to learn whether a webhook works is to post with it, and
		// that would put a test message in somebody's channel.
		config.MessagingWebhook: ErrWebhookUncheckable,
		config.MessagingNone:    ErrNoCredential,
	}[c.creds.Mode()]
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
		return Identity{}, credentialRefusal(identity.Error)
	}

	identity.Granted = grantOf(response.Header)

	return identity, nil
}
