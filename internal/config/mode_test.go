// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestJiraAuthMode(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jira config.Jira
		want config.AuthMode
	}{
		"token only means bearer": {
			jira: config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
			want: config.AuthBearer,
		},
		"token plus user means basic": {
			jira: config.Jira{BaseURL: jiraURL, Token: "t", User: "jacob"},
			want: config.AuthBasic,
		},
		"no token means none": {
			jira: config.Jira{BaseURL: jiraURL, Token: "", User: "jacob"},
			want: config.AuthNone,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tt.jira.AuthMode()
			if got != tt.want {
				t.Errorf("AuthMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthModeString(t *testing.T) {
	t.Parallel()

	cases := map[config.AuthMode]string{
		config.AuthNone:   "none",
		config.AuthBearer: "bearer token",
		config.AuthBasic:  "basic auth",
		// A value outside the enum: the String method must stay total rather than
		// returning an empty string that reads as "no auth configured".
		config.AuthMode(99): "unknown",
	}

	for mode, want := range cases {
		got := mode.String()
		if got != want {
			t.Errorf("AuthMode(%d).String() = %q, want %q", mode, got, want)
		}
	}
}

func TestMissingNamesEveryEmptyRequiredField(t *testing.T) {
	t.Parallel()

	var empty config.Config

	got := empty.Missing()
	// The Slack credential is ONE entry, not two: either transport satisfies it,
	// and naming both as separately missing would read as "set them both".
	want := []string{"jira.base_url", "jira.token", "slack.token or slack.webhook_url"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestMissingIsEmptyForCompleteConfig(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	write(t, workDir, completeConfig)

	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := cfg.Missing()
	if len(got) != 0 {
		t.Errorf("Missing() = %v, want none", got)
	}
}

func TestSlackModeSelectsTheTransport(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		slack config.Slack
		want  config.SlackMode
	}{
		"nothing configured": {
			slack: config.Slack{Token: "", WebhookURL: "", Channel: ""},
			want:  config.SlackNone,
		},
		"bot token": {
			slack: config.Slack{Token: botToken, WebhookURL: "", Channel: devChannel},
			want:  config.SlackBot,
		},
		"webhook only": {
			slack: config.Slack{Token: "", WebhookURL: webhookURL, Channel: ""},
			want:  config.SlackWebhook,
		},
		// Both is not an error. The bot token is the more capable transport, so
		// it wins rather than the configuration being called ambiguous.
		"both prefers the bot token": {
			slack: config.Slack{Token: botToken, WebhookURL: webhookURL, Channel: devChannel},
			want:  config.SlackBot,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tt.slack.Mode()
			if got != tt.want {
				t.Errorf("Mode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSlackModeString(t *testing.T) {
	t.Parallel()

	cases := map[config.SlackMode]string{
		config.SlackNone:    "none",
		config.SlackBot:     "bot token",
		config.SlackWebhook: "incoming webhook",
		// Total, like AuthMode: a value from outside the enum still reads.
		config.SlackMode(99): "unknown",
	}

	for mode, want := range cases {
		if got := mode.String(); got != want {
			t.Errorf("SlackMode(%d).String() = %q, want %q", mode, got, want)
		}
	}
}

func TestMissingAcceptsAWebhookWithoutAChannel(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Jira:  config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
		Slack: config.Slack{Token: "", WebhookURL: webhookURL, Channel: ""},
		Path:  "",
	}

	// An incoming webhook is bound to one channel when it is created, so asking
	// for slack.channel as well would be asking for something with no effect.
	if got := cfg.Missing(); len(got) != 0 {
		t.Errorf("Missing() = %v, want none for a webhook-only configuration", got)
	}
}

func TestMissingRequiresAChannelForABotToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Jira:  config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
		Slack: config.Slack{Token: botToken, WebhookURL: "", Channel: ""},
		Path:  "",
	}

	got := cfg.Missing()
	if strings.Join(got, ",") != "slack.channel" {
		t.Errorf("Missing() = %v, want exactly [slack.channel]", got)
	}
}

func TestSlackTarget(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		slack config.Slack
		want  string
	}{
		"nothing configured": {
			slack: config.Slack{Token: "", WebhookURL: "", Channel: ""},
			want:  "(not set)",
		},
		"bot names its channel": {
			slack: config.Slack{Token: botToken, WebhookURL: "", Channel: devChannel},
			want:  devChannel,
		},
		"bot without a channel says so": {
			slack: config.Slack{Token: botToken, WebhookURL: "", Channel: ""},
			want:  "(no channel set)",
		},
		"webhook describes its binding": {
			slack: config.Slack{Token: "", WebhookURL: webhookURL, Channel: ""},
			want:  "the channel its webhook is bound to",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := tt.slack.Target(); got != tt.want {
				t.Errorf("Target() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlackTargetNeverRevealsTheWebhookURL(t *testing.T) {
	t.Parallel()

	// The channel a webhook posts to is not knowable without calling Slack, and
	// the URL that would reveal it is the credential itself. Target() must
	// describe the binding rather than quote it.
	slack := config.Slack{Token: "", WebhookURL: webhookURL, Channel: ""}

	target := slack.Target()
	if strings.Contains(target, "hooks.slack.com") || strings.Contains(target, "fakefake") {
		t.Errorf("Target() = %q, want it to describe the webhook without quoting it", target)
	}
}
