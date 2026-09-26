// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestRedactedHidesEveryCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		jiraToken  = "jira-token-1234"
		slackToken = "xoxb-slack-token-5678"
	)

	cfg := config.Config{
		Jira: config.Jira{BaseURL: jiraURL, Token: jiraToken, User: ""},
		Messaging: config.Messaging{
			Token:      slackToken,
			WebhookURL: webhookURL,
			Channel:    devChannel,
		},
		Forge: config.Forge{Token: forgeFixture},
		Path:  "/tmp/.workflow.json",
	}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if strings.Contains(redacted.Jira.Token.Reveal(), "jira-token") {
		t.Errorf("jira token leaked: %q", redacted.Jira.Token)
	}

	if strings.Contains(redacted.Messaging.Token.Reveal(), "slack-token") {
		t.Errorf("slack token leaked: %q", redacted.Messaging.Token)
	}

	// Redaction must not mutate the original.
	if cfg.Jira.Token != jiraToken {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Jira.Token)
	}

	if strings.Contains(redacted.Forge.Token.Reveal(), "not-a-real") {
		t.Errorf("forge token leaked: %q", redacted.Forge.Token)
	}

	// A webhook URL is not a URL with a secret in it — it IS the credential.
	// Anyone holding it can post to that channel, so it masks like a token.
	if strings.Contains(redacted.Messaging.WebhookURL.Reveal(), "hooks.slack.com") {
		t.Errorf("webhook url leaked: %q", redacted.Messaging.WebhookURL)
	}

	if cfg.Messaging.WebhookURL != webhookURL {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Messaging.WebhookURL)
	}

	// Enough tail survives to tell two tokens apart.
	if !strings.HasSuffix(redacted.Messaging.Token.Reveal(), "5678") {
		t.Errorf("slack token = %q, want it to end in 5678", redacted.Messaging.Token)
	}
}

func TestRedactedMasksEveryJiraHeaderValue(t *testing.T) {
	t.Parallel()

	// Arrange
	const secret = "cf-access-secret-value"

	cfg := config.Config{Jira: config.Jira{Headers: map[string]config.Secret{"CF-Access-Client-Secret": secret}}}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if shown := redacted.Jira.Headers["CF-Access-Client-Secret"].Reveal(); strings.Contains(shown, secret) {
		t.Errorf("header value leaked: %q", shown)
	}

	// Redaction must not mutate the original map.
	if kept := cfg.Jira.Headers["CF-Access-Client-Secret"].Reveal(); kept != secret {
		t.Errorf("Redacted mutated the original header: %q", kept)
	}
}

func TestRedact(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		secret string
		want   string
	}{
		"empty stays empty":      {secret: "", want: ""},
		"short is fully masked":  {secret: "abcd", want: "****"},
		"long keeps four digits": {secret: "abcdefgh", want: "****efgh"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := config.Redact(tt.secret)

			// Assert
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.secret, got, tt.want)
			}
		})
	}
}

func TestRedactTextMasksEveryCredentialTheConfigurationHolds(t *testing.T) {
	t.Parallel()

	const (
		jiraToken    = "jira-token-1111"
		headerSecret = "cf-secret-2222"
		slackToken   = "xoxb-slack-3333"
	)

	cases := map[string]struct {
		cfg  config.Config
		text string
		want string
	}{
		"a Jira token": {
			cfg:  config.Config{Jira: config.Jira{Token: jiraToken}},
			text: "asked with " + jiraToken, want: "asked with ****1111",
		},
		"a Jira header value": {
			cfg:  config.Config{Jira: config.Jira{Headers: map[string]config.Secret{"CF-Access-Client-Secret": headerSecret}}},
			text: "sent " + headerSecret, want: "sent ****2222",
		},
		"a messaging token": {
			cfg:  config.Config{Messaging: config.Messaging{Token: slackToken}},
			text: "posted with " + slackToken, want: "posted with ****3333",
		},
		"a webhook URL": {
			cfg:  config.Config{Messaging: config.Messaging{WebhookURL: webhookURL}},
			text: "posting to " + webhookURL, want: "posting to ****2468",
		},
		"a forge token": {
			cfg:  config.Config{Forge: config.Forge{Token: forgeFixture}},
			text: "sent " + forgeFixture, want: "sent ****tial",
		},
		// The host stays: masking the userinfo must not lose where it goes.
		"a password in jira.base_url": {
			cfg:  config.Config{Jira: config.Jira{BaseURL: "https://alice:hunter2@jira.example.com"}},
			text: "reading https://alice:hunter2@jira.example.com/rest", want: "reading https://xxxxx@jira.example.com/rest",
		},
		// A header that carries the token is masked whole, not around it.
		"a credential that holds another": {
			cfg: config.Config{Jira: config.Jira{
				Token:   jiraToken,
				Headers: map[string]config.Secret{"Authorization": "Bearer " + jiraToken + "-and-more"},
			}},
			text: "sent Bearer " + jiraToken + "-and-more", want: "sent ****more",
		},
		"no credential at all": {cfg: config.Config{}, text: "the seam failed", want: "the seam failed"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.cfg.RedactText(tt.text)

			// Assert
			if got != tt.want {
				t.Errorf("RedactText(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
