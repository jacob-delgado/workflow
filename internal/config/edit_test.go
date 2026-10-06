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
