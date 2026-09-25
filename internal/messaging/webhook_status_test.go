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
