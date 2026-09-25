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

			// Act
			got := tt.jira.AuthMode()

			// Assert
			if got != tt.want {
				t.Errorf("AuthMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthModeString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mode config.AuthMode
		want string
	}{
		"no auth": {mode: config.AuthNone, want: "none"},
		"bearer":  {mode: config.AuthBearer, want: "bearer token"},
		"basic":   {mode: config.AuthBasic, want: "basic auth"},
		// The String method must stay total rather than returning an empty
		// string that reads as "no auth configured".
		"a value outside the enum": {mode: config.AuthMode(99), want: "unknown"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.mode.String()

			// Assert
			if got != tt.want {
				t.Errorf("AuthMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestMissingNamesWhatTheConfigurationStillNeeds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cfg  config.Config
		want []string
	}{
		// The Slack credential is ONE entry, not two: either transport satisfies
		// it, and naming both as separately missing would read as "set them both".
		// Jira is not required at all: without it the forge's issues are the
		// tracker.
		"an empty configuration names every required field": {
			cfg:  config.Config{},
			want: []string{"messaging.token or messaging.webhook_url"},
		},
		"no jira.base_url leaves the tracker to the forge's issues": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: "", Token: "", User: ""},
				Messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
				Path:      "",
			},
			want: nil,
		},
		"a jira.base_url still needs its token": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: jiraURL, Token: "", User: ""},
				Messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
				Path:      "",
			},
			want: []string{"jira.token"},
		},
		// An incoming webhook is bound to one channel when it is created, so
		// asking for messaging.channel as well would be asking for something with no
		// effect.
		"a webhook needs no channel": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
				Messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
				Path:      "",
			},
			want: nil,
		},
		"a bot token needs a channel": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
				Messaging: config.Messaging{Token: botToken, WebhookURL: "", Channel: ""},
				Path:      "",
			},
			want: []string{"messaging.channel"},
		},
		// forge.kind describes a host, so by itself it describes nothing.
		"a forge kind needs the host it describes": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
				Messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
				Forge:     config.Forge{Kind: "github", Host: "", Token: ""},
				Path:      "",
			},
			want: []string{"forge.host"},
		},
		"a forge kind with its host is complete": {
			cfg: config.Config{
				Jira:      config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
				Messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
				Forge:     config.Forge{Kind: "github", Host: "git.example.com", Token: ""},
				Path:      "",
			},
			want: nil,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.cfg.Missing()

			// Assert
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("Missing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMissingIsEmptyForCompleteConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, completeConfig)

	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Act
	got := cfg.Missing()

	// Assert
	if len(got) != 0 {
		t.Errorf("Missing() = %v, want none", got)
	}
}

func TestMessagingModeString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mode config.MessagingMode
		want string
	}{
		"no transport": {mode: config.MessagingNone, want: "none"},
		"bot":          {mode: config.MessagingBot, want: "bot token"},
		"webhook":      {mode: config.MessagingWebhook, want: "incoming webhook"},
		// Total, like AuthMode: a value from outside the enum still reads.
		"a value outside the enum": {mode: config.MessagingMode(99), want: "unknown"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("MessagingMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestMessagingTarget(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		messaging config.Messaging
		want      string
	}{
		"nothing configured": {
			messaging: config.Messaging{Token: "", WebhookURL: "", Channel: ""},
			want:      "(not set)",
		},
		"bot names its channel": {
			messaging: config.Messaging{Token: botToken, WebhookURL: "", Channel: devChannel},
			want:      devChannel,
		},
		"bot without a channel says so": {
			messaging: config.Messaging{Token: botToken, WebhookURL: "", Channel: ""},
			want:      "(no channel set)",
		},
		"webhook describes its binding": {
			messaging: config.Messaging{Token: "", WebhookURL: webhookURL, Channel: ""},
			want:      "the channel its webhook is bound to",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.messaging.Target(); got != tt.want {
				t.Errorf("Target() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The channel a webhook posts to is not knowable without calling Slack, and the
// URL that would reveal it is the credential itself. Whatever else is set
// alongside a webhook, Target() must describe where posts go without quoting it.
func TestMessagingTargetNeverRevealsTheWebhookURL(t *testing.T) {
	t.Parallel()

	cases := map[string]config.Messaging{
		"a webhook alone":             {Token: "", WebhookURL: webhookURL, Channel: ""},
		"a webhook and a channel":     {Token: "", WebhookURL: webhookURL, Channel: devChannel},
		"a webhook beside a token":    {Token: botToken, WebhookURL: webhookURL, Channel: devChannel},
		"a webhook beside no channel": {Token: botToken, WebhookURL: webhookURL, Channel: ""},
	}

	for name, messaging := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			target := messaging.Target()

			// Assert
			if target == "" || strings.Contains(target, "hooks.slack.com") || strings.Contains(target, "fakefake") {
				t.Errorf("Target() = %q, want it to describe where posts go without quoting the webhook", target)
			}
		})
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw  string
		want string
	}{
		"no credentials is left alone": {
			raw:  jiraURL,
			want: jiraURL,
		},
		"userinfo with a password is masked whole": {
			raw:  "https://alice:sekret@jira.example.com/jira",
			want: "https://xxxxx@jira.example.com/jira",
		},
		"userinfo without a password is masked too": {
			raw:  "https://alice@jira.example.com",
			want: "https://xxxxx@jira.example.com",
		},
		"the rest of the URL is kept": {
			raw:  "https://alice@jira.example.com:8443/jira?os_authType=basic#top",
			want: "https://xxxxx@jira.example.com:8443/jira?os_authType=basic#top",
		},
		"something malformed is returned unchanged": {
			raw:  "://not a url",
			want: "://not a url",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := config.RedactURL(tt.raw)

			// Assert
			if got != tt.want {
				t.Errorf("RedactURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// passwordURL is a base URL someone wrote a password into.
const passwordURL = "https://alice:sekret@jira.example.com"

// maskedPasswordURL is passwordURL as every display of it must read.
const maskedPasswordURL = "https://xxxxx@jira.example.com"

func TestDisplayURLMasksAPasswordAndNamesAnUnsetURL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw  string
		want string
	}{
		"unset":      {raw: "", want: "(not set)"},
		"plain":      {raw: jiraURL, want: jiraURL},
		"a password": {raw: passwordURL, want: maskedPasswordURL},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := config.DisplayURL(tt.raw); got != tt.want {
				t.Errorf("DisplayURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestRedactedMasksAPasswordInTheBaseURL(t *testing.T) {
	t.Parallel()

	// Arrange
	// doctor prints jira.base_url, and its output is what the bug report
	// template invites people to paste into a public issue.
	cfg := config.Config{
		Jira:      config.Jira{BaseURL: passwordURL, Token: "t", User: ""},
		Messaging: config.Messaging{Token: botToken, WebhookURL: "", Channel: devChannel},
		Path:      "",
	}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if redacted.Jira.BaseURL != maskedPasswordURL {
		t.Errorf("the redacted base URL is %q, want %q", redacted.Jira.BaseURL, maskedPasswordURL)
	}

	if cfg.Jira.BaseURL != passwordURL {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Jira.BaseURL)
	}
}
