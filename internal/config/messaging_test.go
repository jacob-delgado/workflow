// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// serviceSlack and serviceWebhook are the display names Service() returns, named
// here so the same literal is not repeated across cases.
const (
	serviceSlack   = "Slack"
	serviceWebhook = "Webhook"
)

func TestMessagingModeReflectsTheKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		messaging config.Messaging
		want      config.MessagingMode
	}{
		"nothing configured": {
			messaging: config.Messaging{},
			want:      config.MessagingNone,
		},
		// A webhook-only kind has no bot token, so one set beside it is ignored and
		// the mode is decided by the webhook alone.
		"teams ignores a token and needs the webhook": {
			messaging: config.Messaging{Kind: config.KindTeams, Token: botToken},
			want:      config.MessagingNone,
		},
		"teams over its webhook": {
			messaging: config.Messaging{Kind: config.KindTeams, WebhookURL: webhookURL},
			want:      config.MessagingWebhook,
		},
		"discord over its webhook": {
			messaging: config.Messaging{Kind: config.KindDiscord, WebhookURL: webhookURL},
			want:      config.MessagingWebhook,
		},
		"a plain webhook": {
			messaging: config.Messaging{Kind: config.KindWebhook, WebhookURL: webhookURL},
			want:      config.MessagingWebhook,
		},
		// Slack is the exception: a bot token is its more capable transport.
		"slack with a bot token": {
			messaging: config.Messaging{Kind: config.KindSlack, Token: botToken, Channel: devChannel},
			want:      config.MessagingBot,
		},
		"slack over its webhook": {
			messaging: config.Messaging{Kind: config.KindSlack, WebhookURL: webhookURL},
			want:      config.MessagingWebhook,
		},
		// Both is not an error. The bot token is the more capable transport, so
		// it wins rather than the configuration being called ambiguous.
		"slack with both prefers the bot token": {
			messaging: config.Messaging{Kind: config.KindSlack, Token: botToken, WebhookURL: webhookURL, Channel: devChannel},
			want:      config.MessagingBot,
		},
		// An empty kind is read as Slack, so a block written before kinds existed
		// still posts with its bot token.
		"an empty kind is slack": {
			messaging: config.Messaging{Token: botToken, Channel: devChannel},
			want:      config.MessagingBot,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.messaging.Mode(); got != tt.want {
				t.Errorf("Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessagingServiceNamesTheKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind config.MessagingKind
		want string
	}{
		"slack":          {kind: config.KindSlack, want: serviceSlack},
		"teams":          {kind: config.KindTeams, want: "Teams"},
		"discord":        {kind: config.KindDiscord, want: "Discord"},
		"a plain kind":   {kind: config.KindWebhook, want: serviceWebhook},
		"empty is slack": {kind: "", want: serviceSlack},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := (config.Messaging{Kind: tt.kind}).Service(); got != tt.want {
				t.Errorf("Service() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMissingNamesOnlyTheWebhookForAWebhookOnlyKind(t *testing.T) {
	t.Parallel()

	// Arrange
	// Teams has no bot token, so the credential it still needs is the webhook
	// URL alone — naming a token would ask for something that would be ignored.
	cfg := config.Config{
		Jira:      config.Jira{BaseURL: jiraURL, Token: "t"},
		Messaging: config.Messaging{Kind: config.KindTeams},
	}

	// Act
	got := cfg.Missing()

	// Assert
	if strings.Join(got, ",") != "messaging.webhook_url" {
		t.Errorf("Missing() = %v, want only messaging.webhook_url", got)
	}
}

func TestProblemsFlagsAnInsecureWebhook(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{
		Messaging: config.Messaging{Kind: config.KindTeams, WebhookURL: "http://hooks.example.com/x"},
	}

	// Act
	got := cfg.Problems()

	// Assert
	if !slices.Contains(got, "messaging.webhook_url is not an https URL") {
		t.Errorf("Problems() = %v, want the plain-http webhook named", got)
	}
}

func TestLoadReadsEveryKnownMessagingKind(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind config.MessagingKind
	}{
		"empty":        {kind: ""},
		"slack":        {kind: config.KindSlack},
		"teams":        {kind: config.KindTeams},
		"discord":      {kind: config.KindDiscord},
		"a plain kind": {kind: config.KindWebhook},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, `{"messaging": {"kind": "`+string(tt.kind)+`", "webhook_url": "`+webhookURL+`"}}`)

			// Act
			cfg, err := config.Load(workDir, t.TempDir())
			// Assert
			if err != nil {
				t.Fatalf("Load returned %v, want messaging.kind %q accepted", err, tt.kind)
			}

			if cfg.Messaging.Kind != tt.kind {
				t.Errorf("messaging.kind = %q, want %q", cfg.Messaging.Kind, tt.kind)
			}
		})
	}
}

func TestAnUnknownMessagingKindIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind string
	}{
		"a service it does not know": {kind: "mastodon"},
		// The kind is read as written, so a capital letter is not folded away.
		"a known kind in another case": {kind: "Teams"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, `{"messaging": {"kind": "`+tt.kind+`", "webhook_url": "`+webhookURL+`"}}`)

			// Act
			_, err := config.Load(workDir, t.TempDir())

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, config.ErrInvalidMessaging) {
				t.Errorf("Load returned %v, want messaging.kind %q refused", err, tt.kind)
			}
		})
	}
}

func TestLoadHintsAtTheSlackToMessagingRename(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file written before the block was renamed still names it "slack", which
	// the strict decoder would otherwise reject with a cryptic unknown-field
	// error rather than a migration the reader can act on.
	workDir := t.TempDir()
	write(t, workDir, `{"slack": {"webhook_url": "`+webhookURL+`"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrSlackRenamed) || !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Load returned %v, want the slack-renamed migration hint", err)
	}
}

func TestRedactedMasksAMessagingWebhookAndToken(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{
		Messaging: config.Messaging{Kind: config.KindTeams, Token: botToken, WebhookURL: webhookURL},
	}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if strings.Contains(redacted.Messaging.WebhookURL.Reveal(), "hooks.slack.com") ||
		redacted.Messaging.Token.Reveal() == botToken {
		t.Errorf("Redacted leaked a messaging secret: %+v", redacted.Messaging)
	}

	if cfg.Messaging.WebhookURL != webhookURL {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Messaging.WebhookURL)
	}
}
