// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package slackauth keeps a Slack user token posting past the twelve hours a
// rotating token lasts: it refreshes the token through Slack's oauth.v2.access,
// keeps each new pair where the configuration says — the macOS keychain or the
// configuration file — and hands out a token with time left on it, refreshing
// under a lock so two processes do not spend the same single-use refresh token.
package slackauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

var (
	// ErrRefreshRefused reports Slack turning a refresh down, in its own code —
	// invalid_refresh_token, bad_client_secret and the like.
	ErrRefreshRefused = errors.New("refreshing the Slack user token was refused")
	// ErrNotAUserToken reports a refresh that answered with a token other than
	// a user token, which is the only kind workflow posts with.
	ErrNotAUserToken = errors.New("refreshing gave no Slack user token")
	// ErrUnreachable reports a refresh that never reached Slack, or an answer
	// that could not be read.
	ErrUnreachable = errors.New("could not reach Slack to refresh the user token")
	// ErrUnexpectedStatus reports a refresh Slack answered with a status other
	// than the 200 its every verdict comes with, or the 429 that asks to wait.
	ErrUnexpectedStatus = errors.New("slack answered the refresh with an unexpected status")
)

// refreshPath is Slack's method that swaps a refresh token for a new pair.
const refreshPath = "/oauth.v2.access"

// answerLimit bounds how much of Slack's answer is read.
const answerLimit = 64 << 10

// Credentials are a Slack user token's rotating credentials: the app's client
// secret, which every refresh is made with, and the access and refresh tokens
// each refresh replaces, with when the access token expires.
type Credentials struct {
	ClientSecret config.Secret
	RefreshToken config.Secret
	AccessToken  config.Secret
	ExpiresAt    time.Time
}

// Refresher refreshes a user token through Slack's API at Base, for the app
// ClientID names, telling the time with Now.
type Refresher struct {
	Do       httpx.Doer
	Base     string
	ClientID string
	Now      func() time.Time
}

// refreshAnswer is the part of Slack's answer a refresh reads.
type refreshAnswer struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Refresh swaps credentials' refresh token for a new pair, sending the client
// ID and secret as Basic authentication, as Slack asks, rather than in the
// form. The refresh token it used is spent, so the pair it returns is the one
// to keep. No credential reaches an error it returns.
func (r Refresher) Refresh(ctx context.Context, credentials Credentials) (Credentials, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {credentials.RefreshToken.Reveal()}}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Base+refreshPath,
		strings.NewReader(form.Encode()))
	if err != nil {
		return Credentials{}, fmt.Errorf("%w: the API address could not be used", ErrUnreachable)
	}

	request.SetBasicAuth(r.ClientID, credentials.ClientSecret.Reveal())
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	answer, err := r.ask(request)
	if err != nil {
		return Credentials{}, err
	}

	return r.renewed(credentials, answer)
}

// ask sends the refresh and reads Slack's answer. Slack gives every verdict,
// a refusal included, with a 200; a 429 asks to wait, and any other status
// says something its body is no verdict on.
func (r Refresher) ask(request *http.Request) (refreshAnswer, error) {
	response, err := r.Do(request)
	if err != nil {
		return refreshAnswer{}, httpx.Unreachable(ErrUnreachable, r.Base, err)
	}
	defer func() { _ = response.Body.Close() }()

	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusTooManyRequests:
		return refreshAnswer{}, httpx.RateLimited(response.Header)
	default:
		return refreshAnswer{}, fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.StatusCode)
	}

	body, err := httpx.Read(response.Body, answerLimit)
	if err != nil {
		return refreshAnswer{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}

	var answer refreshAnswer

	err = json.Unmarshal(body, &answer)
	if err != nil {
		return refreshAnswer{}, fmt.Errorf("%w: its answer was not JSON", ErrUnreachable)
	}

	return answer, nil
}

// renewed is credentials with answer's new pair, or why answer gives none.
func (r Refresher) renewed(credentials Credentials, answer refreshAnswer) (Credentials, error) {
	switch {
	case !answer.OK:
		return Credentials{}, fmt.Errorf("%w: %s", ErrRefreshRefused, slackCode(answer.Error))
	case answer.TokenType != "user":
		return Credentials{}, fmt.Errorf("%w: it answered a %q token", ErrNotAUserToken, slackCode(answer.TokenType))
	case answer.AccessToken == "" || answer.RefreshToken == "" || answer.ExpiresIn <= 0:
		return Credentials{}, fmt.Errorf("%w: its answer left out the new token or its lifetime", ErrRefreshRefused)
	}

	return Credentials{
		ClientSecret: credentials.ClientSecret,
		RefreshToken: config.Secret(answer.RefreshToken),
		AccessToken:  config.Secret(answer.AccessToken),
		ExpiresAt:    r.Now().UTC().Add(time.Duration(answer.ExpiresIn) * time.Second),
	}, nil
}

// slackCode is a code from Slack's answer as it may be shown: Slack's codes are
// short snake_case words, so anything else — which no real answer holds — is
// left out rather than shown.
func slackCode(code string) string {
	if code == "" || len(code) > 64 || strings.Trim(code, "abcdefghijklmnopqrstuvwxyz_0123456789") != "" {
		return "no reason given"
	}

	return code
}
