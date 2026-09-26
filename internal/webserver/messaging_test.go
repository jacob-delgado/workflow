// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestGetMessagingReturnsTheDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Default()
	cfg.Messaging.Token = "xoxb-t"
	cfg.Messaging.Channel = "#dev"

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, filledDeps(), cfg), "/api/messaging"))

	// Assert
	if destination.Service != "Slack" || !destination.Configured ||
		destination.Channel != "#dev" || destination.Author != testAuthor {
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
	destination := decode[api.MessagingDestination](t, get(t, serve(t, filledDeps(), cfg), "/api/messaging"))

	// Assert
	if destination.Service != "Teams" || !destination.Configured || destination.Channel != "" {
		t.Errorf("destination = %+v, want a configured Teams with no channel", destination)
	}
}

func TestGetMessagingHasNoAuthorWithoutAForge(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = nil

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, deps, config.Default()), "/api/messaging"))

	// Assert
	if destination.Author != "" {
		t.Errorf("author = %q, want empty without a forge", destination.Author)
	}
}

func TestGetMessagingHasNoAuthorWhenTheForgeCannotSay(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := filledDeps()
	deps.Author = func() (string, error) { return "", errSeam }

	// Act
	destination := decode[api.MessagingDestination](t, get(t, serve(t, deps, config.Default()), "/api/messaging"))

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
	deps := filledDeps()
	deps.Author = func() (string, error) { return testAuthor, errSeam }

	// Act
	snap := firstSnapshot(t, streamOnce(t, serve(t, deps, config.Default()), "/api/events").Body.String())

	// Assert
	if snap.Messaging.Author != "" {
		t.Errorf("author = %q, want empty when the forge cannot say", snap.Messaging.Author)
	}
}
