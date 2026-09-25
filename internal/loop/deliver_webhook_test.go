// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

func TestDeliverRecordsADiscordPostAnsweredNoContent(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	webhook := config.Messaging{Kind: config.KindDiscord, WebhookURL: config.Secret(server.URL + "/hook")}
	client := messaging.New(server.Client().Do, messaging.APIBase, webhook)
	post := func(channel, text string) error { return client.Post(t.Context(), channel, text) }

	var sent deliveries

	// Act
	err := loop.Deliver(post, sent.memory(), merged())

	// Assert
	if err != nil || !slices.Equal(sent.recorded, []loop.Announced{merged().Made}) {
		t.Errorf("Deliver = %v, recorded %+v; want the post delivered and recorded", err, sent.recorded)
	}
}
