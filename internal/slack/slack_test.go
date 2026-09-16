// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slack_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slack"
)

// botToken is the credential these tests send. Deliberately not shaped like a
// real xoxb- token: gitleaks scans this repository.
const botToken = "slack-bot-token-for-tests"

// okBody is what auth.test answers for a working bot token.
const okBody = `{"ok":true,"url":"https://example.slack.com/","team":"Example",` +
	`"user":"workflow","team_id":"T00000000","user_id":"U00000000"}`

// botCredentials authenticates with a bot token.
func botCredentials() config.Slack {
	return config.Slack{Token: botToken, WebhookURL: "", Channel: "#dev"}
}

// serve starts a Slack API and returns a client pointed at it.
func serve(t *testing.T, handler http.HandlerFunc) slack.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return slack.New(server.Client().Do, server.URL, botCredentials())
}

func TestAuthTestReportsTheWorkspaceAndUser(t *testing.T) {
	t.Parallel()

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(okBody))
	})

	identity, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	if identity.User != "workflow" {
		t.Errorf("User = %q, want %q", identity.User, "workflow")
	}

	if identity.Team != "Example" {
		t.Errorf("Team = %q, want %q", identity.Team, "Example")
	}
}

func TestAuthTestSendsTheTokenOnlyInTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	var (
		gotHeader atomic.Value
		gotURI    atomic.Value
		gotPath   atomic.Value
	)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotHeader.Store(request.Header.Get("Authorization"))
		gotURI.Store(request.RequestURI)
		gotPath.Store(request.URL.Path)

		_, _ = writer.Write([]byte(okBody))
	}))
	t.Cleanup(server.Close)

	client := slack.New(server.Client().Do, server.URL, botCredentials())

	_, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	if got := gotHeader.Load(); got != "Bearer "+botToken {
		t.Errorf("Authorization = %q, want a bearer token", got)
	}

	if got := gotPath.Load(); got != "/auth.test" {
		t.Errorf("path = %q, want /auth.test", got)
	}

	// Slack accepts a token in the query string. That would put it in proxy
	// logs, so it must travel in the header and nowhere else.
	if uri, _ := gotURI.Load().(string); strings.Contains(uri, botToken) {
		t.Errorf("the request URI carried the token: %q", uri)
	}
}

func TestAuthTestRejectsAFailureInsideATwoHundred(t *testing.T) {
	t.Parallel()

	// Slack's own convention: the HTTP status is 200 and the verdict is in the
	// body. Reading the status alone would call a dead token healthy.
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	})

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrRejected) {
		t.Fatalf("AuthTest returned %v, want ErrRejected", err)
	}

	// Slack's own word for what went wrong is the useful part of the message.
	if !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("AuthTest error = %v, want it to carry Slack's reason", err)
	}

	if strings.Contains(err.Error(), botToken) {
		t.Errorf("the error carried the token: %v", err)
	}
}

func TestAuthTestReportsAWebhookAsUncheckable(t *testing.T) {
	t.Parallel()

	// An incoming webhook has no credential endpoint: the only way to learn
	// whether it works is to post with it, which would spam the channel.
	client := slack.New(http.DefaultClient.Do, "https://slack.example.com", config.Slack{
		Token:      "",
		WebhookURL: "https://hooks.slack.com/services/T0/B0/secretpath",
		Channel:    "",
	})

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrWebhookUncheckable) {
		t.Fatalf("AuthTest returned %v, want ErrWebhookUncheckable", err)
	}

	if strings.Contains(err.Error(), "secretpath") || strings.Contains(err.Error(), "hooks.slack.com") {
		t.Errorf("the error quoted the webhook URL: %v", err)
	}
}

func TestAuthTestWithoutACredential(t *testing.T) {
	t.Parallel()

	client := slack.New(http.DefaultClient.Do, "https://slack.example.com", config.Slack{
		Token:      "",
		WebhookURL: "",
		Channel:    "",
	})

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrNoCredential) {
		t.Errorf("AuthTest returned %v, want ErrNoCredential", err)
	}
}

func TestAuthTestTranslatesAnUnexpectedStatus(t *testing.T) {
	t.Parallel()

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrUnexpectedStatus) {
		t.Errorf("AuthTest returned %v, want ErrUnexpectedStatus", err)
	}
}

func TestAuthTestReportsAnUnreachableAPI(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := server.URL
	server.Close()

	client := slack.New(slack.HTTPClient(2*time.Second).Do, base, botCredentials())

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrUnreachable) {
		t.Errorf("AuthTest returned %v, want ErrUnreachable", err)
	}
}

func TestAuthTestReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{not json"))
	})

	_, err := client.AuthTest(t.Context())
	if err == nil {
		t.Fatal("AuthTest accepted a malformed body, want an error")
	}
}

func TestHTTPClientRefusesARedirect(t *testing.T) {
	t.Parallel()

	var secondHopSawHeader atomic.Bool

	second := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		secondHopSawHeader.Store(request.Header.Get("Authorization") != "")

		_, _ = writer.Write([]byte(okBody))
	}))
	t.Cleanup(second.Close)

	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, second.URL+"/auth.test", http.StatusFound)
	}))
	t.Cleanup(first.Close)

	client := slack.New(slack.HTTPClient(5*time.Second).Do, first.URL, botCredentials())

	_, err := client.AuthTest(t.Context())
	if err == nil {
		t.Fatal("AuthTest followed a redirect, want an error")
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}
}

func TestAuthTestReportsAMalformedAPIBase(t *testing.T) {
	t.Parallel()

	// A control character is what url.Parse refuses outright, which is the only
	// way to reach the request-building failure.
	client := slack.New(http.DefaultClient.Do, "https://slack.example.com/\x7f", botCredentials())

	_, err := client.AuthTest(t.Context())
	if !errors.Is(err, slack.ErrUnreachable) {
		t.Fatalf("AuthTest returned %v, want ErrUnreachable", err)
	}

	if strings.Contains(err.Error(), botToken) {
		t.Errorf("the error carried the token: %v", err)
	}
}
