// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// slackApp and slackChannel are the Slack user token's app and channel the
// settings tests save.
const (
	slackApp     = "1234.5678"
	slackChannel = "#dev"
)

const (
	// keptSecret and keptRefresh are the user token's secrets a file already
	// holds; typedRefresh is a refresh token typed into Settings.
	keptSecret   = "client-secret-kept-1111"
	keptRefresh  = "slack-refresh-kept-2222"
	keptAccess   = "xoxe.xoxp-kept-3333"
	typedRefresh = "slack-refresh-typed-4444"
)

// errSlackSaysNo is Slack refusing the secrets Settings typed.
var errSlackSaysNo = errors.New("invalid_refresh_token")

// fileKeepingSlack is a configuration file that keeps the user token's
// secrets itself, as on Linux and Windows, and the configuration read from it.
func fileKeepingSlack(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	cfg.Messaging = config.Messaging{
		Kind: config.KindSlack, ClientID: slackApp, Channel: slackChannel,
		ClientSecret: keptSecret, RefreshToken: keptRefresh, AccessToken: keptAccess,
		ExpiresAt: "2026-09-30T20:00:00Z",
	}

	_, err := config.SaveOver(cfg.Path, cfg, config.Revision{})
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	return cfg
}

// placingNothing is a PlaceSlackCredentials that fails the test if called.
func placingNothing(t *testing.T) func(config.Config) (config.Config, error) {
	t.Helper()

	return func(config.Config) (config.Config, error) {
		t.Error("the Slack secrets were placed")

		return config.Config{}, errSlackSaysNo
	}
}

// typingRefresh is cfg with a new refresh token typed over its own.
func typingRefresh(cfg config.Config) config.Config {
	cfg.Messaging.RefreshToken = typedRefresh

	return cfg
}

// keychainKept is a configuration naming a Slack app whose secrets the
// keychain keeps, with no file written yet.
func keychainKept(t *testing.T) config.Config {
	t.Helper()

	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), config.FileName)
	cfg.Messaging = config.Messaging{Kind: config.KindSlack, ClientID: slackApp, Channel: slackChannel}

	return cfg
}

func TestSlackSecretsSavedInSettingsArePlacedAndKeptOutOfTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	var placed []config.Config

	used := make(chan config.Messaging, 1)
	cfg := config.Default()
	cfg.Path = filepath.Join(t.TempDir(), ".workflow.json")
	deps := webserver.Deps{
		PlaceSlackCredentials: func(withSecrets config.Config) (config.Config, error) {
			placed = append(placed, withSecrets)
			withSecrets.Messaging.ClientSecret, withSecrets.Messaging.RefreshToken = "", ""

			return withSecrets, nil
		},
		UseMessagingSettings: func(settings config.Messaging) { used <- settings },
	}
	handler := serveWith(t, deps, cfg, webserver.Info{Version: testVersion})
	next := cfg
	next.Messaging = config.Messaging{
		Kind: config.KindSlack, ClientID: slackApp, ClientSecret: "client-secret-9999",
		RefreshToken: "slack-refresh-8888", Channel: slackChannel,
	}

	// Act
	saved := putConfig(t, handler, marshal(t, next))

	// Assert
	if saved.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", saved.Code, saved.Body.String())
	}

	if len(placed) != 1 || placed[0].Messaging.RefreshToken.Reveal() != "slack-refresh-8888" {
		t.Fatalf("placed %d configurations, want the one carrying the typed refresh token", len(placed))
	}

	written, err := config.LoadFile(cfg.Path)
	if err != nil || written.Messaging.HoldsUserTokenSecrets() || written.Messaging.ClientID != slackApp {
		t.Errorf("the file reads %+v, %v; want the client ID and no secret", written.Messaging, err)
	}

	if settings := <-used; settings.ClientID != slackApp {
		t.Errorf("the next post would use %+v, want the settings just saved", settings)
	}
}

func TestSlackSecretsSentBackMaskedAreKeptAsTheyWere(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := fileKeepingSlack(t)
	handler := serveWith(t, webserver.Deps{PlaceSlackCredentials: placingNothing(t)}, cfg, webserver.Info{})
	read := get(t, handler, "/api/config")

	// Act
	saved := putConfigOver(t, handler, read.Body.String(), read.Header().Get("ETag"))

	// Assert
	for _, secret := range []string{keptSecret, keptRefresh, keptAccess} {
		if strings.Contains(read.Body.String(), secret) {
			t.Errorf("the read sent a Slack secret back unmasked: %s", read.Body.String())
		}
	}

	written, err := config.LoadFile(cfg.Path)
	held := written.Messaging

	kept := held.ClientSecret == keptSecret && held.RefreshToken == keptRefresh && held.AccessToken == keptAccess &&
		held.ExpiresAt == cfg.Messaging.ExpiresAt
	if saved.Code != http.StatusOK || err != nil || !kept {
		t.Errorf("status %d, file %+v, %v; want the secrets kept as they were", saved.Code, held, err)
	}
}

func TestSlackSecretsTypedIntoAFileThatKeepsThemStartAFreshToken(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := fileKeepingSlack(t)
	handler := serveWith(t, webserver.Deps{PlaceSlackCredentials: placingNothing(t)}, cfg, webserver.Info{})

	// Act
	saved := putConfig(t, handler, marshal(t, typingRefresh(cfg)))

	// Assert
	written, err := config.LoadFile(cfg.Path)
	if saved.Code != http.StatusOK || err != nil {
		t.Fatalf("status %d, %v: %s", saved.Code, err, saved.Body.String())
	}

	held := written.Messaging
	if held.RefreshToken != typedRefresh || held.AccessToken != "" || held.ExpiresAt != "" {
		t.Errorf("the file keeps %+v; want the typed refresh token and no access token, so the next post refreshes", held)
	}
}

func TestASlackRefusalOfTypedSecretsSaysWhatToCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := webserver.Deps{PlaceSlackCredentials: func(config.Config) (config.Config, error) {
		return config.Config{}, fmt.Errorf("%w: %w", messaging.ErrRejected, errSlackSaysNo)
	}}
	cfg := keychainKept(t)
	handler := serveWith(t, deps, cfg, webserver.Info{})

	// Act
	saved := putConfig(t, handler, marshal(t, typingRefresh(cfg)))

	// Assert
	if saved.Code != http.StatusUnprocessableEntity || !strings.Contains(saved.Body.String(), "Slack refused") {
		t.Errorf("status %d: %s; want a 422 saying Slack refused the secrets", saved.Code, saved.Body.String())
	}

	if strings.Contains(saved.Body.String(), typedRefresh) {
		t.Errorf("the answer carries the typed refresh token: %s", saved.Body.String())
	}

	_, err := os.Stat(cfg.Path)
	if err == nil {
		t.Error("the configuration was written though Slack refused the secrets")
	}
}

func TestSlackSecretsAreNotPlacedOverAFileChangedSinceTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := keychainKept(t)
	handler := serveWith(t, webserver.Deps{PlaceSlackCredentials: placingNothing(t)}, cfg, webserver.Info{})
	etag := get(t, handler, "/api/config").Header().Get("ETag")

	err := os.WriteFile(cfg.Path, []byte(`{"version":"1"}`), 0o600)
	if err != nil {
		t.Fatalf("editing the file: %v", err)
	}

	// Act
	saved := putConfigOver(t, handler, marshal(t, typingRefresh(cfg)), etag)

	// Assert
	if saved.Code != http.StatusConflict {
		t.Errorf("status %d: %s; want 409 before the refresh token is spent", saved.Code, saved.Body.String())
	}
}
