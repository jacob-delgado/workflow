// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// The app and the pair a refresh starts from and the one Slack answers with.
const (
	clientID     = "1234.5678"
	clientSecret = "client-secret-9999"
	oldRefresh   = "xoxe-1-old-refresh"
	oldAccess    = "xoxe.xoxp-1-old-access"
	newRefresh   = "xoxe-1-new-refresh"
	newAccess    = "xoxe.xoxp-1-new-access"
)

// errNoRoute stands in for a network that does not reach Slack.
var errNoRoute = errors.New("dial tcp: no route")

// testNow is the time the refresh is made at.
func testNow() time.Time {
	return time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
}

// asked is what one request to the fake Slack carried.
type asked struct {
	path, user, password string
	form                 url.Values
}

// fakeSlack answers oauth.v2.access with body, recording what it was asked.
func fakeSlack(t *testing.T, body string) (*httptest.Server, func() []asked) {
	t.Helper()

	var (
		lock sync.Mutex
		seen []asked
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user, password, _ := request.BasicAuth()
		raw, _ := io.ReadAll(request.Body)
		form, _ := url.ParseQuery(string(raw))

		lock.Lock()

		seen = append(seen, asked{path: request.URL.Path, user: user, password: password, form: form})

		lock.Unlock()

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server, func() []asked {
		lock.Lock()
		defer lock.Unlock()

		return append([]asked(nil), seen...)
	}
}

// refresherFor is a Refresher that asks server.
func refresherFor(server *httptest.Server) slackauth.Refresher {
	return slackauth.Refresher{Do: server.Client().Do, Base: server.URL, ClientID: clientID, Now: testNow}
}

// startingPair is the credentials a refresh starts from.
func startingPair() slackauth.Credentials {
	return slackauth.Credentials{
		ClientSecret: clientSecret, RefreshToken: oldRefresh, AccessToken: oldAccess, ExpiresAt: testNow(),
	}
}

// rotated is Slack's answer to a refresh of a user token.
const rotated = `{"ok":true,"access_token":"` + newAccess + `","token_type":"user","expires_in":43200,` +
	`"refresh_token":"` + newRefresh + `","scope":"chat:write"}`

func TestRefreshSwapsThePairForSlacksNewOne(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)

	// Act
	refreshed, err := refresherFor(server).Refresh(t.Context(), startingPair())
	// Assert
	if err != nil {
		t.Fatalf("Refresh = %v, want the new pair", err)
	}

	want := slackauth.Credentials{
		ClientSecret: clientSecret, RefreshToken: newRefresh, AccessToken: newAccess,
		ExpiresAt: testNow().Add(12 * time.Hour),
	}
	if refreshed != want {
		t.Errorf("Refresh = %+v, want %+v", refreshed, want)
	}

	request := seen()[0]
	if request.path != "/oauth.v2.access" || request.user != clientID || request.password != clientSecret {
		t.Errorf("asked %s as %q, want oauth.v2.access with the app's client ID and secret in Basic auth",
			request.path, request.user)
	}

	if request.form.Get("grant_type") != "refresh_token" || request.form.Get("refresh_token") != oldRefresh ||
		request.form.Has("client_secret") {
		t.Errorf("form = %v, want the refresh grant and token, and the secret kept out of it", request.form)
	}
}

func TestRefreshRefusesATokenThatIsNoUsers(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := fakeSlack(t, strings.Replace(rotated, `"token_type":"user"`, `"token_type":"bot"`, 1))

	// Act
	_, err := refresherFor(server).Refresh(t.Context(), startingPair())

	// Assert
	if !errors.Is(err, slackauth.ErrNotAUserToken) {
		t.Errorf("Refresh = %v, want %v", err, slackauth.ErrNotAUserToken)
	}
}

func TestRefreshReportsSlacksRefusalWithoutASecret(t *testing.T) {
	t.Parallel()

	// Arrange
	server, _ := fakeSlack(t, `{"ok":false,"error":"invalid_refresh_token"}`)

	// Act
	_, err := refresherFor(server).Refresh(t.Context(), startingPair())

	// Assert
	if !errors.Is(err, slackauth.ErrRefreshRefused) || !strings.Contains(err.Error(), "invalid_refresh_token") {
		t.Fatalf("Refresh = %v, want Slack's refusal named", err)
	}

	for _, secret := range []string{clientSecret, oldRefresh, oldAccess} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("the error %q carries a credential", err)
		}
	}
}

func TestRefreshReportsASlackThatDidNotAnswer(t *testing.T) {
	t.Parallel()

	// Arrange
	refresher := slackauth.Refresher{
		Do:   func(*http.Request) (*http.Response, error) { return nil, errNoRoute },
		Base: "https://slack.example.com/api", ClientID: clientID, Now: testNow,
	}

	// Act
	_, err := refresher.Refresh(t.Context(), startingPair())

	// Assert
	if !errors.Is(err, slackauth.ErrUnreachable) {
		t.Errorf("Refresh = %v, want %v", err, slackauth.ErrUnreachable)
	}
}
