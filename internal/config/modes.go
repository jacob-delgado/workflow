// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
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
	// KindSlack posts to Slack, over a bot token or an incoming webhook.
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
// it post as Slack.
func (c Config) validateMessaging() error {
	if !c.Messaging.Kind.Known() {
		return fmt.Errorf("%w: kind is slack, teams, discord or webhook: %q",
			ErrInvalidMessaging, c.Messaging.Kind)
	}

	return nil
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

// webhookOnly reports a kind whose only transport is an incoming webhook: it has
// no bot token, so a token set beside it is ignored. Slack is the exception —
// it can post with a bot token too.
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
	// MessagingBot posts with a bot token, which needs a channel and an
	// invitation. Only Slack has one; the other services post over a webhook.
	MessagingBot
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
	case MessagingBot:
		return "bot token"
	case MessagingWebhook:
		return "incoming webhook"
	default:
		return "unknown"
	}
}
