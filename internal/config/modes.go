// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import "fmt"

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

// String names the authentication mode for humans.
var _ fmt.Stringer = AuthMode(0)

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

// SlackMode is the transport a message to Slack travels over.
type SlackMode int

const (
	// SlackNone means no Slack credential is configured.
	SlackNone SlackMode = iota
	// SlackBot posts with a bot token, which needs a channel and an invitation.
	SlackBot
	// SlackWebhook posts to an incoming webhook, which carries its own channel.
	SlackWebhook
)

// String names the transport for humans.
var _ fmt.Stringer = SlackMode(0)

func (s SlackMode) String() string {
	switch s {
	case SlackNone:
		return "none"
	case SlackBot:
		return "bot token"
	case SlackWebhook:
		return "incoming webhook"
	default:
		return "unknown"
	}
}
