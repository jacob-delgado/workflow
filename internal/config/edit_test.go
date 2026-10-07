// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// The Jira token homeFile holds, and the credentials the tests type over the
// masks.
const (
	homeToken    = "jira-token-home-1234"
	typedToken   = "typed-token-5678"
	typedRefresh = "xoxe-1-typed"
)

// readHome writes homeFile alone and reads it back, as an editor is seeded.
func readHome(t *testing.T) (config.Files, config.Config, config.Revision) {
	t.Helper()

	files := config.Files{Home: write(t, t.TempDir(), homeFile)}

	cfg, revision, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the home file: %v", err)
	}

	return files, cfg, revision
}

// reread reads the files again, as the next session would.
func reread(t *testing.T, files config.Files) config.Config {
	t.Helper()

	cfg, _, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}

	return cfg
}

func TestSaveEditKeepsAStoredTokenSentBackMasked(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Project = ossProject

	// Act
	_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

	// Assert
	saved := reread(t, files)
	if err != nil || saved.Jira.Token.Reveal() != homeToken || saved.Jira.Project != ossProject {
		t.Errorf("saved project %q, token kept %t, %v; want the project changed and the token kept",
			saved.Jira.Project, saved.Jira.Token.Reveal() == homeToken, err)
	}
}

func TestSaveEditTakesATokenTypedInPlaceOfTheMask(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken

	// Act
	_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

	// Assert
	if saved := reread(t, files); err != nil || saved.Jira.Token.Reveal() != typedToken {
		t.Errorf("saved the typed token: %t, %v; want it saved", saved.Jira.Token.Reveal() == typedToken, err)
	}
}

func TestSaveEditRefusesFilesChangedSinceTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)

	err := os.WriteFile(files.Home, []byte(`{"jira": {"project": "ELSE"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("changing the file: %v", err)
	}

	// Act
	_, _, err = config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: read.Redacted()})

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) {
		t.Errorf("err = %v, want ErrChangedOnDisk", err)
	}
}

func TestSaveEditHandsTypedSlackSecretsToBePlaced(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.RefreshToken = typedRefresh

	var handed config.Secret

	place := func(cfg config.Config) (config.Config, error) {
		handed = cfg.Messaging.RefreshToken
		cfg.Messaging.RefreshToken = ""

		return cfg, nil
	}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, PlaceSlackCredentials: place,
	})

	// Assert
	if saved := reread(t, files); err != nil || handed.Reveal() != typedRefresh || saved.Messaging.RefreshToken != "" {
		t.Errorf("handed the typed token: %t, file keeps it: %t, %v; want it placed and not in the file",
			handed.Reveal() == typedRefresh, saved.Messaging.RefreshToken != "", err)
	}
}

func TestSaveEditWithNowhereToPlaceSlackSecretsKeepsThemInTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.ClientSecret = "typed-client-secret"
	edited.Messaging.AccessToken = "xoxe.xoxp-stale"

	// Act
	_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

	// Assert
	saved, readErr := os.ReadFile(files.Home)
	if err != nil || readErr != nil || !strings.Contains(string(saved), "typed-client-secret") ||
		strings.Contains(string(saved), "xoxe.xoxp-stale") {
		t.Errorf("saved %v, %v; want the typed secret in the file and no access token", err, readErr)
	}
}

func TestSaveEditPlacesNothingOverFilesChangedSinceTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.RefreshToken = typedRefresh
	placed := false

	err := os.WriteFile(files.Home, []byte(`{"jira": {"project": "ELSE"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("changing the file: %v", err)
	}

	// Act
	_, _, err = config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(cfg config.Config) (config.Config, error) {
			placed = true

			return cfg, nil
		},
	})

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) || placed {
		t.Errorf("err = %v, placed %t; want ErrChangedOnDisk and the typed token unspent", err, placed)
	}
}

func TestKeepStoredTakesABaseURLEditedFromItsMask(t *testing.T) {
	t.Parallel()

	// Arrange
	stored := config.Default()
	stored.Jira.BaseURL = "https://ana:hunter2@jira.example.com"
	incoming := stored.Redacted()
	incoming.Jira.BaseURL = "https://jira.example.org"

	// Act
	kept := config.KeepStored(incoming, stored, nil)

	// Assert
	if kept.Jira.BaseURL != "https://jira.example.org" {
		t.Errorf("base URL = %q, want the edited one", kept.Jira.BaseURL)
	}
}

func TestKeepStoredDropsJiraHeadersTheEditRemoved(t *testing.T) {
	t.Parallel()

	// Arrange
	stored := config.Default()
	stored.Jira.Headers = map[string]config.Secret{"X-Auth": "header-dropped-5678"}
	incoming := stored.Redacted()
	incoming.Jira.Headers = nil

	// Act
	kept := config.KeepStored(incoming, stored, nil)

	// Assert
	if kept.Jira.Headers != nil {
		t.Errorf("headers = %v, want none: the edit removed them", kept.Jira.Headers)
	}
}

func TestSaveEditOverAFileKeepingSlackSecretsKeepsTheTypedOnesThere(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: write(t, t.TempDir(), slackUserHome)}

	read, revision, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the home file: %v", err)
	}

	edited := read.Redacted()
	edited.Messaging.RefreshToken = typedRefresh
	placed := false

	// Act
	_, _, err = config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(cfg config.Config) (config.Config, error) {
			placed = true

			return cfg, nil
		},
	})

	// Assert
	saved := reread(t, files)
	if err != nil || placed || saved.Messaging.RefreshToken.Reveal() != typedRefresh || saved.Messaging.AccessToken != "" {
		t.Errorf("err %v, placed %t, file keeps typed %t, access token %t; want the typed token kept in the "+
			"file, unplaced, and no access token", err, placed, saved.Messaging.RefreshToken.Reveal() == typedRefresh,
			saved.Messaging.AccessToken != "")
	}
}

func TestSaveEditPlacesNothingOverFilesItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.RefreshToken = typedRefresh
	placed := false

	err := os.Remove(files.Home)
	if err == nil {
		err = os.Mkdir(files.Home, 0o700)
	}

	if err != nil {
		t.Fatalf("putting a directory where the file was: %v", err)
	}

	// Act
	_, _, err = config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(cfg config.Config) (config.Config, error) {
			placed = true

			return cfg, nil
		},
	})

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) || placed {
		t.Errorf("err = %v, placed %t; want ErrChangedOnDisk and the typed token unspent", err, placed)
	}
}

func TestSaveEditRemovingTheJiraTokenWritesAFileWithNoToken(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: read.Redacted(),
		Removed: []config.Credential{config.CredentialJiraToken},
	})

	// Assert
	written, readErr := os.ReadFile(files.Home)
	if err != nil || readErr != nil || strings.Contains(string(written), homeToken) || reread(t, files).Jira.Token != "" {
		t.Errorf("err %v, %v; the file still holds the token: %t; want it written without the token",
			err, readErr, strings.Contains(string(written), homeToken))
	}
}

func TestKeepStoredClearsEachRemovedCredential(t *testing.T) {
	t.Parallel()

	held := func(cfg config.Config) map[config.Credential]config.Secret {
		return map[config.Credential]config.Secret{
			config.CredentialJiraToken:    cfg.Jira.Token,
			config.CredentialForgeToken:   cfg.Forge.Token,
			config.CredentialWebhookURL:   cfg.Messaging.WebhookURL,
			config.CredentialClientSecret: cfg.Messaging.ClientSecret,
			config.CredentialRefreshToken: cfg.Messaging.RefreshToken,
		}
	}

	stored := config.Default()
	stored.Jira.Token, stored.Forge.Token = "jira-token-1111", "forge-token-2222"
	stored.Messaging.WebhookURL = "https://hooks.example.com/3333"
	stored.Messaging.ClientSecret, stored.Messaging.RefreshToken = "client-secret-4444", "xoxe-1-5555"

	for removed := range held(stored) {
		t.Run(string(removed), func(t *testing.T) {
			t.Parallel()

			// Act
			kept := held(config.KeepStored(stored.Redacted(), stored, []config.Credential{removed}))

			// Assert
			for credential, value := range kept {
				if want := held(stored)[credential]; credential == removed && value != "" ||
					credential != removed && value != want {
					t.Errorf("%s kept %t; want only %s cleared", credential, value != "", removed)
				}
			}
		})
	}
}

func TestKeepStoredRemovingASlackSecretTakesTheAccessTokenWithIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		removed config.Credential
		other   func(config.Config) config.Secret
	}{
		{config.CredentialRefreshToken, func(cfg config.Config) config.Secret { return cfg.Messaging.ClientSecret }},
		{config.CredentialClientSecret, func(cfg config.Config) config.Secret { return cfg.Messaging.RefreshToken }},
	}

	for _, test := range tests {
		t.Run(string(test.removed), func(t *testing.T) {
			t.Parallel()

			// Arrange
			stored := config.Default()
			stored.Messaging.ClientSecret, stored.Messaging.RefreshToken = "client-secret-4444", "xoxe-1-5555"
			stored.Messaging.AccessToken, stored.Messaging.ExpiresAt = "xoxe.xoxp-6666", "2026-01-01T00:00:00Z"

			// Act
			kept := config.KeepStored(stored.Redacted(), stored, []config.Credential{test.removed})

			// Assert
			if kept.Messaging.AccessToken != "" || kept.Messaging.ExpiresAt != "" || test.other(kept) != test.other(stored) {
				t.Errorf("access token kept %t, expiry %q, the other secret kept %t; want the access token and "+
					"its expiry gone with %s, the other secret kept", kept.Messaging.AccessToken != "",
					kept.Messaging.ExpiresAt, test.other(kept) == test.other(stored), test.removed)
			}
		})
	}
}

func TestSaveEditRemovingTheSlackSecretsPlacesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: write(t, t.TempDir(), slackUserHome)}

	read, revision, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the home file: %v", err)
	}

	placed := false

	// Act
	_, _, err = config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: read.Redacted(),
		Removed: []config.Credential{config.CredentialClientSecret, config.CredentialRefreshToken},
		PlaceSlackCredentials: func(cfg config.Config) (config.Config, error) {
			placed = true

			return cfg, nil
		},
	})

	// Assert
	saved := reread(t, files)
	if err != nil || placed || saved.Messaging.HoldsUserTokenSecrets() {
		t.Errorf("err %v, placed %t, file holds the user token's secrets %t; want them gone, unplaced",
			err, placed, saved.Messaging.HoldsUserTokenSecrets())
	}
}
