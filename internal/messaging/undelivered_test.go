// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// errConnectionReset is a post that never got an answer.
var errConnectionReset = errors.New("connection reset by peer")

// unreachable is a transport that never gets an answer.
func unreachable(*http.Request) (*http.Response, error) {
	return nil, errConnectionReset
}

// postAnswer is a post's answer: a status and its body, or none at all.
type postAnswer struct {
	status int
	body   string
	lost   bool
}

// doer is the transport that answers every post as the answer says.
func (a postAnswer) doer() messaging.Doer {
	if a.lost {
		return unreachable
	}

	return answeringWith(a.status, a.body)
}

// eachAnswer is a post answered every way that does or may not deliver it,
// and whether it then surely went undelivered.
func eachAnswer() map[string]struct {
	answer      postAnswer
	undelivered bool
} {
	return map[string]struct {
		answer      postAnswer
		undelivered bool
	}{
		"a message refused":    {answer: postAnswer{status: http.StatusBadRequest, body: "too long"}, undelivered: true},
		"a credential refused": {answer: postAnswer{status: http.StatusForbidden}, undelivered: true},
		"a rate limit":         {answer: postAnswer{status: http.StatusTooManyRequests}, undelivered: true},
		"a server error":       {answer: postAnswer{status: http.StatusBadGateway}, undelivered: false},
		"no answer":            {answer: postAnswer{lost: true}, undelivered: false},
		"a redirect status":    {answer: postAnswer{status: http.StatusMultipleChoices}, undelivered: false},
	}
}

func TestUndeliveredTellsAWebhookPostSurelyNotDelivered(t *testing.T) {
	t.Parallel()

	kinds := []config.MessagingKind{config.KindSlack, config.KindTeams, config.KindDiscord, config.KindWebhook}

	for _, kind := range kinds {
		for name, tt := range eachAnswer() {
			t.Run(string(kind)+": "+name, func(t *testing.T) {
				t.Parallel()

				// Arrange
				webhook := messagingWebhook(kind, "https://hooks.example.com/hook")
				client := messaging.New(tt.answer.doer(), messaging.APIBase, webhook)

				// Act
				err := client.Post(t.Context(), "", message)

				// Assert
				if err == nil || messaging.Undelivered(err) != tt.undelivered {
					t.Errorf("Post = %v, Undelivered = %t; want an error, undelivered %t",
						err, messaging.Undelivered(err), tt.undelivered)
				}
			})
		}
	}
}

func TestUndeliveredTellsASlackUserPostSurelyNotDelivered(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		answer      postAnswer
		undelivered bool
	}{
		"a channel refused": {
			answer: postAnswer{status: http.StatusOK, body: `{"ok":false,"error":"is_archived"}`}, undelivered: true,
		},
		"a token refused": {
			answer: postAnswer{status: http.StatusOK, body: `{"ok":false,"error":"token_revoked"}`}, undelivered: true,
		},
		"a rate limit":         {answer: postAnswer{status: http.StatusTooManyRequests}, undelivered: true},
		"a server error":       {answer: postAnswer{status: http.StatusInternalServerError}, undelivered: false},
		"no answer":            {answer: postAnswer{lost: true}, undelivered: false},
		"an answer unreadable": {answer: postAnswer{status: http.StatusOK, body: "ok"}, undelivered: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := messaging.New(tt.answer.doer(), messaging.APIBase, userCredentials()).WithToken(heldToken)

			// Act
			err := client.Post(t.Context(), "#dev", message)

			// Assert
			if err == nil || messaging.Undelivered(err) != tt.undelivered {
				t.Errorf("Post = %v, Undelivered = %t; want an error, undelivered %t",
					err, messaging.Undelivered(err), tt.undelivered)
			}
		})
	}
}

func TestUndeliveredTellsAPostThatNeverLeft(t *testing.T) {
	t.Parallel()

	cases := map[string]config.Messaging{
		"no credential":          {Kind: config.KindSlack},
		"a webhook not on https": messagingWebhook(config.KindSlack, "http://hooks.example.com/hook"),
	}

	for name, creds := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := messaging.New(answering(http.StatusOK), messaging.APIBase, creds)

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if err == nil || !messaging.Undelivered(err) {
				t.Errorf("Post = %v, Undelivered = %t; want it surely undelivered", err, messaging.Undelivered(err))
			}
		})
	}
}

func TestUndeliveredIsFalseForNoError(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if messaging.Undelivered(nil) {
		t.Error("Undelivered(nil) = true, want false: nothing failed")
	}
}
