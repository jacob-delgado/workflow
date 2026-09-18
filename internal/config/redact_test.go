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
		Slack: config.Slack{
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

	if strings.Contains(redacted.Slack.Token.Reveal(), "slack-token") {
		t.Errorf("slack token leaked: %q", redacted.Slack.Token)
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
	if strings.Contains(redacted.Slack.WebhookURL.Reveal(), "hooks.slack.com") {
		t.Errorf("webhook url leaked: %q", redacted.Slack.WebhookURL)
	}

	if cfg.Slack.WebhookURL != webhookURL {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Slack.WebhookURL)
	}

	// Enough tail survives to tell two tokens apart.
	if !strings.HasSuffix(redacted.Slack.Token.Reveal(), "5678") {
		t.Errorf("slack token = %q, want it to end in 5678", redacted.Slack.Token)
	}
}

func TestRedactedMasksEveryJiraHeaderValue(t *testing.T) {
	t.Parallel()

	// Arrange
	const secret = "cf-access-secret-value"

	cfg := config.Config{Jira: config.Jira{Headers: map[string]string{"CF-Access-Client-Secret": secret}}}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if strings.Contains(redacted.Jira.Headers["CF-Access-Client-Secret"], secret) {
		t.Errorf("header value leaked: %q", redacted.Jira.Headers["CF-Access-Client-Secret"])
	}

	// Redaction must not mutate the original map.
	if cfg.Jira.Headers["CF-Access-Client-Secret"] != secret {
		t.Errorf("Redacted mutated the original header: %q", cfg.Jira.Headers["CF-Access-Client-Secret"])
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
