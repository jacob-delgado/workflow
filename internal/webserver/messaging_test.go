// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestGetMessagingReturnsTheDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.ClientID = slackApp
	cfg.Messaging.Channel = slackChannel

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(), cfg),
		"/api/messaging"))

	// Assert
	if destination.Service != "Slack" || !destination.Configured ||
		destination.Channel != slackChannel || destination.Author != webtest.TestAuthor {
		t.Errorf("destination = %+v, want configured Slack, #dev and octocat", destination)
	}
}

func TestGetMessagingMarksAWebhookServiceConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	// A Teams webhook is fully configured yet carries no channel, so the
	// destination must report it configured without one.
	cfg := config.Default()
	cfg.Messaging.Kind = "teams"
	cfg.Messaging.WebhookURL = "https://example.com/hook"

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, webtest.Serve(t, webtest.FilledDeps(), cfg),
		"/api/messaging"))

	// Assert
	if destination.Service != "Teams" || !destination.Configured || destination.Channel != "" {
		t.Errorf("destination = %+v, want a configured Teams with no channel", destination)
	}
}

func TestGetMessagingHasNoAuthorWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Forge.Author = nil

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()),
		"/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty without a forge", destination.Author)
	}
}

func TestGetMessagingHasNoAuthorWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webtest.FilledDeps()
	deps.Forge.Author = func(context.Context) (string, error) { return "", webtest.ErrSeam }

	// Act
	destination := webtest.Decode[api.MessagingDestination](t, webtest.Get(t, webtest.Serve(t, deps, config.Default()),
		"/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty when the forge cannot say", destination.Author)
	}
}

func TestSnapshotHasNoAuthorWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	// The failing read still names someone, so only the error can empty the
	// author.
	deps := webtest.FilledDeps()
	deps.Forge.Author = func(context.Context) (string, error) { return webtest.TestAuthor, webtest.ErrSeam }

	// Act
	snap := webtest.FirstSnapshot(t, webtest.StreamOnce(t, webtest.Serve(t, deps, config.Default()),
		"/api/events").Body.String())

	// Assert
	if snap.Messaging.Author != "" {
		t.Errorf("author = %q, want empty when the forge cannot say", snap.Messaging.Author)
	}
}
