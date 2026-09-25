// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestABotPostRefusedForItsTokenIsARejectedCredential(t *testing.T) {
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
		"a bot not in the channel": {
			code: "not_in_channel", want: refused, notWant: rejected, says: "the bot is not in #dev",
		},
		"a code it does not explain": {code: "msg_too_long", want: refused, notWant: rejected, says: "msg_too_long"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server, _ := slackReceiving(t, http.StatusOK, `{"ok":false,"error":"`+tt.code+`"}`)
			client := messaging.New(server.Client().Do, server.URL, botCredentials())

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
