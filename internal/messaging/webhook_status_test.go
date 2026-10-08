// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// answering is a transport that answers every post with status and no body.
func answering(status int) messaging.Doer {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	}
}

func TestAWebhookPostIsDeliveredOnAny2xxAnswer(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status int
		want   error
	}{
		"the last 2xx":  {status: http.StatusMultipleChoices - 1, want: nil},
		"a 1xx":         {status: http.StatusSwitchingProtocols, want: messaging.ErrUnexpectedStatus},
		"the first 3xx": {status: http.StatusMultipleChoices, want: messaging.ErrUnexpectedStatus},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			webhook := messagingWebhook(config.KindWebhook, "https://hooks.example.com/hook")
			client := messaging.New(answering(tt.status), messaging.APIBase, webhook)

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Post answered %d returned %v, want %v", tt.status, err, tt.want)
			}
		})
	}
}

// answeringWith is a transport that answers every post with status and body.
func answeringWith(status int, body string) messaging.Doer {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
}

func TestAWebhookRefusalBlamesTheCredentialOnlyWhenItIsTheCredential(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind   config.MessagingKind
		status int
		body   string
		want   error
	}{
		"discord: a message too long": {
			kind: config.KindDiscord, status: http.StatusBadRequest,
			body: `{"content":["Must be 2000 or fewer in length."]}`, want: messaging.ErrPostRefused,
		},
		"teams: a payload it cannot read": {
			kind: config.KindTeams, status: http.StatusBadRequest,
			body: "Bad payload received by generic incoming webhook.", want: messaging.ErrPostRefused,
		},
		"slack: an archived channel": {
			kind: config.KindSlack, status: http.StatusGone, body: "channel_is_archived", want: messaging.ErrPostRefused,
		},
		"a token not accepted": {
			kind: config.KindDiscord, status: http.StatusUnauthorized,
			body: `{"message":"Invalid Webhook Token","code":50027}`, want: messaging.ErrRejected,
		},
		"a post forbidden": {
			kind: config.KindWebhook, status: http.StatusForbidden, body: "forbidden", want: messaging.ErrRejected,
		},
		"an address with nothing behind it": {
			kind: config.KindDiscord, status: http.StatusNotFound,
			body: `{"message":"Unknown Webhook","code":10015}`, want: messaging.ErrRejected,
		},
		"slack: a workspace gone": {
			kind: config.KindSlack, status: http.StatusGone, body: "team_disabled", want: messaging.ErrRejected,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			webhook := messagingWebhook(tt.kind, "https://hooks.example.com/hook/secret-part")
			client := messaging.New(answeringWith(tt.status, tt.body), messaging.APIBase, webhook)

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, messaging.ErrRejected) == errors.Is(err, messaging.ErrPostRefused) {
				t.Errorf("Post answered %d %q = %v, want %v alone", tt.status, tt.body, err, tt.want)
			}
		})
	}
}

func TestAWebhookRefusalNeverQuotesTheWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	// A plain webhook server's default 404 quotes the path it was asked, and a
	// webhook's address is its credential.
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("Cannot POST " + request.URL.Path + " at " + "https://" + request.Host +
			request.URL.Path + "?" + request.URL.RawQuery))
	}))
	t.Cleanup(server.Close)

	address := server.URL + "/hooks/secret-part-123?sig=signed-part-456"
	client := messaging.New(server.Client().Do, messaging.APIBase, messagingWebhook(config.KindWebhook, address))

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "Cannot POST") {
		t.Fatalf("Post = %v, want the server's reason", err)
	}

	for _, secret := range []string{address, "/hooks/secret-part-123", "secret-part", "signed-part"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("Post = %v, which quotes %q", err, secret)
		}
	}
}

// visibleTail is how many of a secret's last characters config.Redact shows.
const visibleTail = 4

// quotedFragment is the first run of secret, one longer than the tail
// config.Redact shows, that text quotes.
func quotedFragment(text, secret string) (string, bool) {
	for start := 0; start+visibleTail < len(secret); start++ {
		if fragment := secret[start : start+visibleTail+1]; strings.Contains(text, fragment) {
			return fragment, true
		}
	}

	return "", false
}

func TestAWebhookRefusalNeverQuotesTheWebhookWherePageIsCut(t *testing.T) {
	t.Parallel()

	// Everything after the path's first segment is the credential.
	const hookTail = "T0001/B0002/abcdefghijklmnopqrstuvwx"

	address := "https://hooks.example.com/hooks/" + hookTail
	quote := "Cannot POST /hooks/" + hookTail

	// However much comes before the quote — text, the whole address quoted
	// first, or controls that are taken out — the cut can fall inside it.
	cases := map[string]string{}
	for padding := range 700 {
		cases[fmt.Sprintf("after %d bytes of text", padding)] = strings.Repeat(".", padding) + quote
		cases[fmt.Sprintf("after the address and %d bytes", padding)] = address + " " + strings.Repeat(".", padding) + quote
		cases[fmt.Sprintf("after %d controls", padding)] = strings.Repeat("\x1b[m", padding) + quote
	}

	for name, page := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			webhook := messagingWebhook(config.KindWebhook, address)
			client := messaging.New(answeringWith(http.StatusNotFound, page), messaging.APIBase, webhook)

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, messaging.ErrRejected) {
				t.Fatalf("Post = %v, want %v", err, messaging.ErrRejected)
			}

			if fragment, quoted := quotedFragment(err.Error(), hookTail); quoted {
				t.Errorf("Post = %q, which quotes %q of the webhook", err, fragment)
			}
		})
	}
}

func TestAWebhookRefusalIsCutShort(t *testing.T) {
	t.Parallel()

	// Arrange
	// An error page of a megabyte is no reason to print whole.
	page := strings.Repeat("<p>refused</p>", 1<<16)
	webhook := messagingWebhook(config.KindWebhook, "https://hooks.example.com/hook")
	client := messaging.New(answeringWith(http.StatusBadRequest, page), messaging.APIBase, webhook)

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrPostRefused) || len(err.Error()) > 1024 {
		t.Errorf("Post = %d bytes of error, want the refusal's reason cut short", len(err.Error()))
	}
}

func TestAPostRefusedWithNoReasonSaysItsStatus(t *testing.T) {
	t.Parallel()

	cases := map[string]config.Messaging{
		"through a webhook":   messagingWebhook(config.KindWebhook, "https://hooks.example.com/hook"),
		"with a user's token": userCredentials(),
	}

	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := messaging.New(answering(http.StatusBadRequest), messaging.APIBase, creds).WithToken(heldToken)

			// Act
			err := client.Post(t.Context(), "#dev", message)

			// Assert
			if !errors.Is(err, messaging.ErrPostRefused) || !strings.Contains(err.Error(), "status 400") {
				t.Errorf("Post = %v, want the message refused, naming the status", err)
			}
		})
	}
}
