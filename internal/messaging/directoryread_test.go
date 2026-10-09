// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestChannelIDReportsAReadSlackRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	client := slackAnswering(t, `{"ok":false,"error":"invalid_auth"}`)

	// Act
	id, err := client.ChannelID(t.Context(), "dev")

	// Assert
	if !errors.Is(err, messaging.ErrRejected) || id != "" {
		t.Errorf("ChannelID = %q, %v; want no channel and ErrRejected", id, err)
	}
}

func TestADirectoryReadOfAnAnswerItCannotReadSaysSo(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an answer that is not JSON": `{not json`,
		// Slack's verdict reads, but the listing under it is of another shape.
		"a listing of the wrong shape": `{"ok":true,"channels":"dev"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := slackAnswering(t, body)

			// Act
			id, err := client.ChannelID(t.Context(), "dev")

			// Assert
			if err == nil || !strings.Contains(err.Error(), "reading the answer from Slack") || id != "" {
				t.Errorf("ChannelID = %q, %v; want no channel and the answer unread", id, err)
			}
		})
	}
}

func TestADirectoryReadOfAMalformedAPIBaseIsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent atomic.Bool

	client := messaging.New(counting(&sent), "https://slack.example.com/\x7f", userCredentials()).WithToken(heldToken)

	// Act
	_, err := client.Users(t.Context())

	// Assert
	if !errors.Is(err, messaging.ErrUnreachable) || sent.Load() || strings.Contains(err.Error(), "slack.example.com") {
		t.Errorf("Users = %v (sent %t), want ErrUnreachable before sending, without the address", err, sent.Load())
	}
}
