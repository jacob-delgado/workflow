// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// botToken is the credential these tests send. Deliberately not shaped like a
// real xoxb- token: gitleaks scans this repository.
const botToken = "slack-bot-token-for-tests"

// okBody is what auth.test answers for a working bot token.
const okBody = `{"ok":true,"url":"https://example.slack.com/","team":"Example",` +
	`"user":"workflow","team_id":"T00000000","user_id":"U00000000"}`

// botCredentials authenticates with a bot token.
func botCredentials() config.Messaging {
	return config.Messaging{Token: botToken, WebhookURL: "", Channel: "#dev"}
}

// serve starts a Slack API and returns a client pointed at it.
func serve(t *testing.T, handler http.HandlerFunc) messaging.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return messaging.New(server.Client().Do, server.URL, botCredentials())
}

func TestAuthTestReportsTheWorkspaceAndUser(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(okBody))
	})

	// Act
	identity, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	// Assert
	if identity.User != "workflow" {
		t.Errorf("User = %q, want %q", identity.User, "workflow")
	}

	if identity.Team != "Example" {
		t.Errorf("Team = %q, want %q", identity.Team, "Example")
	}
}

func TestAuthTestSendsTheTokenOnlyInTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	// Arrange
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

	client := messaging.New(server.Client().Do, server.URL, botCredentials())

	// Act
	_, err := client.AuthTest(t.Context())
	if err != nil {
		t.Fatalf("AuthTest returned %v, want nil", err)
	}

	// Assert
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

	// Arrange
	// Slack's own convention: the HTTP status is 200 and the verdict is in the
	// body. Reading the status alone would call a dead token healthy.
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	})

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrRejected) {
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

func TestAuthTestRefusesWhatItCannotCheck(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		base        string
		credentials config.Messaging
		want        error
		// hidden is what the error must never quote.
		hidden []string
	}{
		// An incoming webhook has no credential endpoint: the only way to learn
		// whether it works is to post with it, which would spam the channel.
		"a webhook": {
			base: "https://slack.example.com",
			credentials: config.Messaging{
				Token: "", WebhookURL: "https://hooks.slack.com/services/T0/B0/secretpath", Channel: "",
			},
			want:   messaging.ErrWebhookUncheckable,
			hidden: []string{"secretpath", "hooks.slack.com"},
		},
		"no credential": {
			base:        "https://slack.example.com",
			credentials: config.Messaging{Token: "", WebhookURL: "", Channel: ""},
			want:        messaging.ErrNoCredential,
		},
		// A control character is what url.Parse refuses outright, which is the
		// only way to reach the request-building failure.
		"an API base url.Parse refuses": {
			base:        "https://slack.example.com/\x7f",
			credentials: botCredentials(),
			want:        messaging.ErrUnreachable,
			hidden:      []string{botToken},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := messaging.New(http.DefaultClient.Do, tt.base, tt.credentials)

			// Act
			_, err := client.AuthTest(t.Context())

			// Assert
			if !errors.Is(err, tt.want) {
				t.Fatalf("AuthTest returned %v, want %v", err, tt.want)
			}

			for _, hidden := range tt.hidden {
				if strings.Contains(err.Error(), hidden) {
					t.Errorf("the error quoted %q: %v", hidden, err)
				}
			}
		})
	}
}

func TestAuthTestTranslatesAnUnexpectedStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	})

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrUnexpectedStatus) {
		t.Errorf("AuthTest returned %v, want ErrUnexpectedStatus", err)
	}
}

func TestAuthTestTellsRateLimitingApart(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, httpx.ErrRateLimited) {
		t.Errorf("AuthTest returned %v, want ErrRateLimited", err)
	}
}

func TestAuthTestReportsAnUnreachableAPI(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := server.URL
	server.Close()

	client := messaging.New(httpx.Client(2*time.Second).Do, base, botCredentials())

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrUnreachable) {
		t.Errorf("AuthTest returned %v, want ErrUnreachable", err)
	}
}

func TestAuthTestReportsAnUnreadableBody(t *testing.T) {
	t.Parallel()

	// Arrange
	client := serve(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{not json"))
	})

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if _, isSyntax := errors.AsType[*json.SyntaxError](err); !isSyntax {
		t.Errorf("AuthTest returned %v, want the malformed body's syntax error", err)
	}
}

func TestAuthTestRefusesARedirect(t *testing.T) {
	t.Parallel()

	// Arrange
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

	client := messaging.New(httpx.Client(5*time.Second).Do, first.URL, botCredentials())

	// Act
	_, err := client.AuthTest(t.Context())

	// Assert
	if !errors.Is(err, httpx.ErrRedirected) {
		t.Errorf("AuthTest returned %v, want ErrRedirected", err)
	}

	if secondHopSawHeader.Load() {
		t.Error("the credential was forwarded to the redirect target")
	}
}
