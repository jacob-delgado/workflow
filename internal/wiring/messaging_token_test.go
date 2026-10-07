// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The messaging service posts to Slack with a rotating user token, which every
// post asks for anew, so a token that ran out between two posts is refreshed
// rather than sent stale. A post these tests make never reaches Slack: its
// token is refused before anything is sent. Each keeps the token's
// credentials in the configuration file, which every system can hold, so no
// test reads the keychain of the machine it runs on.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// slackChannel is the channel a user-token configuration posts to.
const slackChannel = "#dev"

// halfLoggedIn is a configuration file that keeps the user token's credentials
// itself, holding the app's client secret and nothing to post with yet.
func halfLoggedIn(t *testing.T) config.Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)
	contents := `{"messaging": {"kind": "slack", "client_id": "1234.5678", "client_secret": "client-secret-9999",` +
		` "channel": "` + slackChannel + `"}}`

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("loading the configuration: %v", err)
	}

	return cfg
}

func TestAUserTokenNotSetUpIsReportedBeforeAnythingIsSent(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)
	seams := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Messaging

	// Act
	err := seams.Post("", "hi")

	// Assert
	if !errors.Is(err, messaging.ErrNoCredential) || !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("Post = %v, want no credential, and the login named", err)
	}
}

func TestMessagingSettingsSavedWhileRunningReachTheNextPost(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := halfLoggedIn(t)
	deps, controls := wiring.Deps(t.Context(), cfg, wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Act
	controls.UseMessagingSettings(config.Messaging{Kind: config.KindSlack, WebhookURL: "http://hooks.example.com/x"})

	err := deps.Messaging.Post("", "hi")

	// Assert
	if !errors.Is(err, messaging.ErrInsecureWebhook) {
		t.Errorf("Post = %v, want the webhook saved since used, and refused for its address", err)
	}
}
