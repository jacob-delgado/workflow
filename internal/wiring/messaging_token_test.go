// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The messaging service finds a Slack bot token on first use, as Jira does, so a
// command that never posts never runs messaging.token_command. A post these
// tests make never reaches Slack: its token is refused before anything is sent,
// which the request log, left empty, shows.

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// slackChannel is the channel a bot configuration posts to.
const slackChannel = "#dev"

func TestTheMessagingTokenCommandRunsOnEachPostThatNeedsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newFailingTokenCommand(t)
	cfg := config.Default()
	cfg.Messaging = config.Messaging{TokenCommand: tokens.command, Channel: slackChannel}

	// Act: wire the seams
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Assert: the token command has not run
	if runs := tokens.runs(); runs != 0 {
		t.Fatalf("wiring the seams ran the messaging token command %d times, want none", runs)
	}

	// Act: post twice
	firstErr := seams.Post("", "hello")
	secondErr := seams.Post("", "hello")

	// Assert: each post looked for the token again, and neither was sent
	if !errors.Is(firstErr, messaging.ErrNoCredential) || !errors.Is(secondErr, messaging.ErrNoCredential) {
		t.Errorf("Post = %v, then %v; want messaging.ErrNoCredential both times", firstErr, secondErr)
	}

	if runs := tokens.runs(); runs != 2 {
		t.Errorf("two posts ran the failing messaging token command %d times, want once each", runs)
	}
}

func TestAMessagingTokenSourceThatGivesNoTokenIsReportedBeforeAnythingIsSent(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		settings config.Messaging
		want     string
	}{
		"a command that fails": {
			settings: config.Messaging{TokenCommand: failingCommand, Channel: slackChannel},
			want:     "token command",
		},
		"a command that prints nothing": {
			settings: config.Messaging{TokenCommand: silentCommand, Channel: slackChannel},
			want:     commandSource,
		},
		"an environment variable that is not set": {
			settings: config.Messaging{TokenEnv: unsetVariable, Channel: slackChannel},
			want:     unsetVariable,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Default()
			cfg.Messaging = tt.settings
			// Should a post be sent after all, it gives up at once rather than
			// waiting on Slack.
			cfg.Timing.RequestTimeout = "1ms"

			var logged strings.Builder

			requestLog := wiring.NewRequestLog(&logged, nil)
			seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, requestLog).Messaging

			// Act
			err := seams.Post("", "hello")

			// Assert
			if !errors.Is(err, messaging.ErrNoCredential) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Post = %v, want messaging.ErrNoCredential naming %q", err, tt.want)
			}

			if sent := logged.String(); sent != "" {
				t.Errorf("the post was sent without a token: %q", sent)
			}
		})
	}
}

func TestResolvingAheadRunsTheMessagingTokenCommandOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := newTokenCommand(t)
	cfg := config.Default()
	cfg.Messaging = config.Messaging{TokenCommand: tokens.command, Channel: slackChannel}
	_, resolveAhead := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	resolveAhead()
	resolveAhead()

	// Assert
	if runs := tokens.runs(); runs != 1 {
		t.Errorf("resolving ahead twice ran the messaging token command %d times, want once", runs)
	}
}
