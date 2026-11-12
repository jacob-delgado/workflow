// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// A login keeps every piece of the Slack user token in one file: the client
// ID beside the secrets, in the file the secrets are read from, so every
// directory that reads the secrets reads the app they are for too.

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

// renewedRefresh is the refresh token slackRefreshing's Slack gives back.
const renewedRefresh = "xoxe-1-next"

// homeMessaging is the messaging block of the home file alone, as a directory
// with no file of its own reads it.
func homeMessaging(t *testing.T, home string) config.Messaging {
	t.Helper()

	cfg, _, err := config.LoadLayersAt(config.Files{Home: filepath.Join(home, config.FileName)})
	if err != nil {
		t.Fatalf("reading the home file: %v", err)
	}

	return cfg.Messaging
}

func TestSlackLoginInARepositoryNamesTheAppBesideItsSecretsAtHome(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}
	clitest.WriteFile(t, where.Home, `{}`)
	repository := clitest.WriteFile(t, where.Dir, slackUserTokenFile)
	slackRefreshing(t, func() {})

	// Act
	printed, err := clitest.RunStreamsAt(t, where, loginTyped(), "slack", "login")

	// Assert
	held := homeMessaging(t, where.Home)
	if err != nil || held.ClientID != clitest.SlackClientID || held.RefreshToken.Reveal() != renewedRefresh {
		t.Errorf("slack login = %v (%+v); the home file names the app %q beside its refresh token %t; want the "+
			"client ID kept with the secrets", err, printed, held.ClientID, held.RefreshToken.Reveal() == renewedRefresh)
	}

	unchanged(t, repository, slackUserTokenFile)
}

func TestSlackLoginNeverKeepsTheSecretsWithoutTheApp(t *testing.T) {
	t.Parallel()

	// Arrange
	// The repository's file stops parsing while Slack answers the refresh.
	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}
	clitest.WriteFile(t, where.Home, `{}`)
	repository := clitest.WriteFile(t, where.Dir, slackUserTokenFile)
	slackRefreshing(t, clitest.BreakFile(t, repository))

	// Act
	_, err := clitest.RunStreamsAt(t, where, loginTyped(), "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), repository) {
		t.Errorf("slack login = %v, want it to name the file it could not read back, %s", err, repository)
	}

	held := homeMessaging(t, where.Home)
	if held.RefreshToken.Reveal() != renewedRefresh || held.ClientID != clitest.SlackClientID {
		t.Errorf("the home file keeps the refresh token Slack gave back %t with the client ID %q; want both kept "+
			"there together", held.RefreshToken.Reveal() == renewedRefresh, held.ClientID)
	}
}

func TestSlackLoginRefusesARepositoryNamingAnotherApp(t *testing.T) {
	t.Parallel()

	// Arrange
	// The repository's own client ID would win over the one the login names
	// beside the secrets at home.
	const otherApp = `{"messaging": {"kind": "slack", "client_id": "9999.0000", "channel": "#dev"}}`

	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}
	home := clitest.WriteFile(t, where.Home, `{}`)
	repository := clitest.WriteFile(t, where.Dir, otherApp)

	var refreshed atomic.Bool

	slackRefreshing(t, func() { refreshed.Store(true) })

	// Act
	_, err := clitest.RunStreamsAt(t, where, loginTyped(), "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), repository) || refreshed.Load() {
		t.Errorf("slack login = %v, refreshing the token typed %t; want it refused before any refresh, "+
			"naming %s, whose client ID would win over the one logged in to", err, refreshed.Load(), repository)
	}

	wantExit(t, err, 3)
	unchanged(t, home, `{}`)
}
