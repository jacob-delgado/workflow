// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The messaging seams read the Slack directory whenever the settings in
// effect post with a Slack user token, and answer ErrNoCredential otherwise,
// following a switch of settings either way without a restart.

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

func TestTheDirectorySeamsAreBoundForASlackUserToken(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)

	// Act
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Assert
	if seams.ChannelMembers == nil || seams.UserGroups == nil || seams.RefreshDirectory == nil {
		t.Error("a directory seam is nil under a Slack user token")
	}
}

func TestAWebhookBindsADirectoryThatHasNoCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	// Settings may switch to a Slack user token while workflow runs, so the
	// seams are there; until then they answer ErrNoCredential.
	cfg := config.Config{Messaging: config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example.com/x"}}
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	if seams.UserGroups == nil {
		t.Fatal("no directory seam is bound for a webhook, so a switch to a user token could not tag")
	}

	_, err := seams.UserGroups()

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups under a webhook = %v, want ErrNoCredential without asking for a token", err)
	}
}

func TestSettingsSwitchedToASlackUserTokenReadTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// The configuration file is the one Settings saves the user token's
	// settings to; workflow started with a webhook.
	cfg := halfLoggedIn(t)
	userToken := cfg.Messaging
	cfg.Messaging = config.Messaging{Kind: config.KindSlack, WebhookURL: "https://hooks.example.com/x"}
	deps, controls := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	controls.UseMessagingSettings(userToken)

	// Act
	_, err := deps.Messaging.UserGroups()

	// Assert
	if !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups after switching to a user token = %v, want the token asked for", err)
	}
}

func TestSettingsSwitchedAwayFromSlackStopReadingTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	deps, controls := wiring.Deps(t.Context(), halfLoggedIn(t), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	controls.UseMessagingSettings(config.Messaging{Kind: config.KindTeams, WebhookURL: "https://teams.example.com/x"})

	// Act
	_, err := deps.Messaging.UserGroups()

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("UserGroups after switching to Teams = %v, want ErrNoCredential without asking for a token", err)
	}
}
