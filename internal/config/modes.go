// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidMessaging reports a messaging block naming a service this build does
// not post to.
var ErrInvalidMessaging = errors.New("invalid messaging")

// AuthMode is how a request authenticates to Jira.
type AuthMode int

const (
	// AuthNone means no credentials are configured.
	AuthNone AuthMode = iota
	// AuthBearer sends the token as a bearer token, which is what a Jira Data
	// Center personal access token expects.
	AuthBearer
	// AuthBasic sends user and token as HTTP Basic credentials.
	AuthBasic
)

var _ fmt.Stringer = AuthMode(0)

// String names the authentication mode for humans.
func (a AuthMode) String() string {
	switch a {
	case AuthNone:
		return "none"
	case AuthBearer:
		return "bearer token"
	case AuthBasic:
		return "basic auth"
	default:
		return "unknown"
	}
}

// MessagingKind is the service a messaging block posts to. Empty is read as
// Slack, so a block written before more services existed still works.
type MessagingKind string

const (
	// KindSlack posts to Slack, with a rotating user token or over an incoming
	// webhook.
	KindSlack MessagingKind = "slack"
	// KindTeams posts to a Microsoft Teams incoming webhook.
	KindTeams MessagingKind = "teams"
	// KindDiscord posts to a Discord webhook.
	KindDiscord MessagingKind = "discord"
	// KindWebhook posts plain text to any incoming webhook.
	KindWebhook MessagingKind = "webhook"
)

// Known reports whether k is a kind this build understands. An empty kind is
// known: it is read as Slack for a block written before kinds existed.
func (k MessagingKind) Known() bool {
	switch k {
	case "", KindSlack, KindTeams, KindDiscord, KindWebhook:
		return true
	default:
		return false
	}
}

// validateMessaging refuses a kind this build does not post to, rather than let
// it post as Slack; a Slack block that sets up both of its transports, which
// is a choice left unmade; a user token on a kind that has none; and an
// expiry that is no time.
func (c Config) validateMessaging() error {
	messaging := c.Messaging

	switch {
	case !messaging.Kind.Known():
		return fmt.Errorf("%w: kind is slack, teams, discord or webhook: %q", ErrInvalidMessaging, messaging.Kind)
	case messaging.hasUserToken() && messaging.Kind.webhookOnly():
		return fmt.Errorf("%w: client_id and the user token belong to Slack alone, not %s",
			ErrInvalidMessaging, messaging.Kind.Service())
	case messaging.hasUserToken() && messaging.WebhookURL != "":
		return fmt.Errorf("%w: Slack posts with a user token or a webhook_url, not both; remove one",
			ErrInvalidMessaging)
	case messaging.ExpiresAt != "" && !isTimestamp(messaging.ExpiresAt):
		return fmt.Errorf("%w: expires_at is not an RFC 3339 time: %q", ErrInvalidMessaging, messaging.ExpiresAt)
	default:
		return nil
	}
}

// isTimestamp reports a value written as an RFC 3339 time.
func isTimestamp(value string) bool {
	_, err := time.Parse(time.RFC3339, value)

	return err == nil
}

// Service names the messaging service for display: the pane title and doctor.
func (k MessagingKind) Service() string {
	switch k {
	case KindTeams:
		return "Teams"
	case KindDiscord:
		return "Discord"
	case KindWebhook:
		return "Webhook"
	case KindSlack:
		return "Slack"
	default:
		// Parse refuses every other kind, so this is the empty one, read as Slack.
		return "Slack"
	}
}

// webhookOnly reports a kind whose only transport is an incoming webhook. Slack
// is the exception — it can post with a user token instead.
func (k MessagingKind) webhookOnly() bool {
	switch k {
	case KindTeams, KindDiscord, KindWebhook:
		return true
	case KindSlack:
		return false
	default:
		// Parse refuses every other kind, so this is the empty one, read as Slack.
		return false
	}
}

// MessagingMode is the transport a message travels over.
type MessagingMode int

const (
	// MessagingNone means no messaging credential is configured.
	MessagingNone MessagingMode = iota
	// MessagingUser posts with a Slack user token, as you, to a channel you can
	// post in. Only Slack has one; the other services post over a webhook.
	MessagingUser
	// MessagingWebhook posts to an incoming webhook, which carries its own
	// channel.
	MessagingWebhook
)

var _ fmt.Stringer = MessagingMode(0)

// String names the transport for humans.
func (m MessagingMode) String() string {
	switch m {
	case MessagingNone:
		return "none"
	case MessagingUser:
		return "user token"
	case MessagingWebhook:
		return "incoming webhook"
	default:
		return "unknown"
	}
}
