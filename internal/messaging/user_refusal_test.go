// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestAUserPostRefusedForItsTokenIsARejectedCredential(t *testing.T) {
	t.Parallel()

	rejected, refused := messaging.ErrRejected, messaging.ErrPostRefused

	// Slack answers a post whose token it will not take the way it answers any
	// other refusal — 200 and ok:false — so the code alone says which to fix:
	// the credential, or the message and its channel.
	cases := map[string]struct {
		code    string
		want    error
		notWant error
		says    string
	}{
		"no token sent":         {code: "not_authed", want: rejected, notWant: refused, says: "not_authed"},
		"a token not valid":     {code: "invalid_auth", want: rejected, notWant: refused, says: "invalid_auth"},
		"a deactivated account": {code: "account_inactive", want: rejected, notWant: refused, says: "account_inactive"},
		"a revoked token":       {code: "token_revoked", want: rejected, notWant: refused, says: "token_revoked"},
		"an expired token":      {code: "token_expired", want: rejected, notWant: refused, says: "token_expired"},
		"a scope the app lacks": {code: "missing_scope", want: rejected, notWant: refused, says: "missing_scope"},
		"you not in the channel": {
			code: "not_in_channel", want: refused, notWant: rejected, says: "you are not in #dev",
		},
		"a code it does not explain": {code: "msg_too_long", want: refused, notWant: rejected, says: "msg_too_long"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"`+tt.code+`"}`)
			client := messaging.New(server.Client().Do, server.URL, userCredentials()).WithToken(heldToken)

			// Act
			err := client.Post(t.Context(), "", message)

			// Assert
			if !errors.Is(err, tt.want) || errors.Is(err, tt.notWant) {
				t.Fatalf("Post answered %s returned %v, want %v and never %v", tt.code, err, tt.want, tt.notWant)
			}

			if !strings.Contains(err.Error(), tt.says) {
				t.Errorf("Post answered %s returned %q, want it to say %q", tt.code, err, tt.says)
			}
		})
	}
}

func TestARefusalThatBreaksOffGivesItsStatusForItsReason(t *testing.T) {
	t.Parallel()

	// Arrange
	// The start of the reason arrives, then the connection drops: a reason
	// cut where the connection happened to fail could say the opposite of
	// what was meant, so none of it is shown.
	breaking := func(*http.Request) (*http.Response, error) {
		body := io.MultiReader(strings.NewReader("message not too lo"), brokenBody{})

		return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(body)}, nil
	}
	client := messaging.New(breaking, messaging.APIBase, userCredentials()).WithToken(heldToken)

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrPostRefused) || !strings.Contains(err.Error(), "status 400") ||
		strings.Contains(err.Error(), "too lo") {
		t.Errorf("Post = %v, want the refusal told by its status alone", err)
	}
}

func TestARefusalNeverShowsAWebhookThatDoesNotParse(t *testing.T) {
	t.Parallel()

	// Arrange
	// Settings holding a webhook beside the user token: one that does not
	// parse is the credential all the same, should a refusal quote it.
	const webhook = "https://hooks.slack.com/services/T0/B0/%zz-unescaped-secret"

	settings := userCredentials()
	settings.WebhookURL = config.Secret(webhook)

	server, _ := slackReceiving(t, http.StatusBadRequest, "refused, as was "+webhook)
	client := messaging.New(server.Client().Do, server.URL, settings).WithToken(heldToken)

	// Act
	err := client.Post(t.Context(), "", message)

	// Assert
	if !errors.Is(err, messaging.ErrPostRefused) || !strings.Contains(err.Error(), "refused, as was") {
		t.Fatalf("Post = %v, want the refusal's reason", err)
	}

	if strings.Contains(err.Error(), "unescaped-secret") {
		t.Errorf("Post = %q, want the webhook masked", err)
	}
}
