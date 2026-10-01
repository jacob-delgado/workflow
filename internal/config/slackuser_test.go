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

// A Slack app's rotating user token: the app's client ID and secret, and the
// access and refresh tokens workflow keeps swapping.
const (
	slackClientID     = "1234.5678"
	slackClientSecret = "client-secret-9999"
	slackRefreshToken = "xoxe-1-refresh-8888"
	slackAccessToken  = "xoxe.xoxp-1-access-7777"
)

func TestASlackUserTokenPostsAsTheUser(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"messaging": {"kind": "slack", "client_id": "`+slackClientID+`", "channel": "#dev"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())

	// Assert
	if err != nil || cfg.Messaging.Mode() != config.MessagingUser {
		t.Fatalf("Load = %v, mode %v; want the user token", err, cfg.Messaging.Mode())
	}

	missing := cfg.Missing()
	if slices.ContainsFunc(missing, func(field string) bool { return strings.HasPrefix(field, "messaging") }) {
		t.Errorf("Missing() = %q, want nothing of messaging's", missing)
	}
}

func TestASlackUserTokenAndAWebhookTogetherAreRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"messaging": {"kind": "slack", "client_id": "`+slackClientID+`",`+
		` "webhook_url": "`+webhookURL+`", "channel": "#dev"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalidMessaging) || !strings.Contains(err.Error(), "not both") {
		t.Errorf("Load = %v, want the two Slack transports refused together", err)
	}
}

func TestTheUserTokenSettingsBelongToSlackAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"messaging": {"kind": "teams", "webhook_url": "`+webhookURL+`",`+
		` "client_id": "`+slackClientID+`"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalidMessaging) {
		t.Errorf("Load = %v, want a Slack client_id refused on Teams", err)
	}
}

func TestAnExpiryThatIsNotATimeIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"messaging": {"kind": "slack", "client_id": "`+slackClientID+`", "expires_at": "tomorrow"}}`)

	// Act
	_, err := config.Load(workDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrInvalidMessaging) {
		t.Errorf("Load = %v, want an expires_at that is no time refused", err)
	}
}

func TestMissingNamesTheSlackLoginOrTheWebhook(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		messaging config.Messaging
		want      string
	}{
		"nothing set up": {
			messaging: config.Messaging{Kind: config.KindSlack},
			want:      "messaging.client_id (workflow slack login) or messaging.webhook_url",
		},
		"a user token with no channel": {
			messaging: config.Messaging{Kind: config.KindSlack, ClientID: slackClientID},
			want:      "messaging.channel",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			missing := config.Config{Messaging: tt.messaging}.Missing()

			// Assert
			if !slices.Contains(missing, tt.want) {
				t.Errorf("Missing() = %q, want it to name %q", missing, tt.want)
			}
		})
	}
}

// userTokenConfig is a configuration whose file holds every Slack user-token
// secret.
func userTokenConfig() config.Config {
	return config.Config{Messaging: config.Messaging{
		Kind: config.KindSlack, ClientID: slackClientID, ClientSecret: slackClientSecret,
		RefreshToken: slackRefreshToken, AccessToken: slackAccessToken,
	}}
}

func TestRedactedMasksTheSlackUserTokensSecrets(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := userTokenConfig()

	// Act
	redacted := cfg.Redacted().Messaging

	// Assert
	for name, secret := range map[string]config.Secret{
		"client_secret": redacted.ClientSecret, "refresh_token": redacted.RefreshToken, "access_token": redacted.AccessToken,
	} {
		if !strings.HasPrefix(secret.Reveal(), "****") {
			t.Errorf("Redacted %s = %q, want it masked", name, secret.Reveal())
		}
	}

	if redacted.ClientID != slackClientID {
		t.Errorf("Redacted client_id = %q, want it shown: it is no secret", redacted.ClientID)
	}
}

func TestRedactTextMasksTheSlackUserTokensSecrets(t *testing.T) {
	t.Parallel()

	// Arrange
	text := "refreshed " + slackRefreshToken + " with " + slackClientSecret + " for " + slackAccessToken

	// Act
	masked := userTokenConfig().RedactText(text)

	// Assert
	for _, secret := range []string{slackRefreshToken, slackClientSecret, slackAccessToken} {
		if strings.Contains(masked, secret) {
			t.Errorf("RedactText left %q in %q", secret, masked)
		}
	}
}
