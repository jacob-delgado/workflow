// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

// A credential typed into Settings while working in a repository would land
// in the repository's file, a file in a working tree one `git add -A` from a
// commit. Settings writes none there: an edit that would is refused, and
// writes nothing.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// readLayered writes homeFile and a repository's file over it and reads the
// pair, as an editor working in the repository is seeded.
func readLayered(t *testing.T, repo string) (config.Files, config.Config, config.Revision) {
	t.Helper()

	files := layered(t, repo)

	cfg, revision, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("reading the layers: %v", err)
	}

	return files, cfg, revision
}

func TestSaveEditWritesNoTypedCredentialIntoTheRepositoryFile(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*config.Config){
		"a Jira token":   func(cfg *config.Config) { cfg.Jira.Token = typedToken },
		"a forge token":  func(cfg *config.Config) { cfg.Forge.Token = typedToken },
		"a webhook":      func(cfg *config.Config) { cfg.Messaging.WebhookURL = "https://hooks.slack.example/" + typedToken },
		"a Jira header":  func(cfg *config.Config) { cfg.Jira.Headers = map[string]config.Secret{"X-Gateway": typedToken} },
		"a Slack secret": func(cfg *config.Config) { cfg.Messaging.ClientSecret = typedToken },
	}

	for name, typeIn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files, read, revision := readLayered(t, `{"jira": {"project": "OSS"}}`)
			edited := read.Redacted()
			typeIn(&edited)

			// Act
			_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

			// Assert
			written, readErr := os.ReadFile(files.Repo)
			if !errors.Is(err, config.ErrCredentialInRepository) || readErr != nil ||
				strings.Contains(string(written), typedToken) {
				t.Errorf("SaveEdit = %v; the repository's file holds %q (%v); want ErrCredentialInRepository "+
					"and no typed credential there", err, written, readErr)
			}
		})
	}
}

func TestSaveEditPlacesNoSlackSecretsWhenAnotherCredentialIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, `{"jira": {"project": "OSS"}}`)
	edited := read.Redacted()
	edited.Jira.Token, edited.Messaging.RefreshToken = typedToken, typedRefresh

	placed := false
	place := func(cfg config.Config) (config.Config, error) {
		placed = true

		return cfg, nil
	}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, PlaceSlackCredentials: place,
	})

	// Assert
	if !errors.Is(err, config.ErrCredentialInRepository) || placed {
		t.Errorf("SaveEdit = %v, placed %t; want the Jira token refused before the refresh token is spent", err, placed)
	}
}

func TestSaveEditKeepsTheSlackSecretsPlacedOutsideTheRepositoryFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, `{"jira": {"project": "OSS"}}`)
	edited := read.Redacted()
	edited.Messaging.RefreshToken, edited.Messaging.Channel = typedRefresh, "#releases"

	placed := false
	keychain := func(cfg config.Config) (config.Config, error) {
		placed = true
		cfg.Messaging.ClientSecret, cfg.Messaging.RefreshToken = "", ""

		return cfg, nil
	}

	// Act
	_, saved, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, PlaceSlackCredentials: keychain,
	})

	// Assert
	written, readErr := os.ReadFile(files.Repo)
	if err != nil || readErr != nil || !placed || saved == revision ||
		reread(t, files).Messaging.Channel != "#releases" || strings.Contains(string(written), typedRefresh) {
		t.Errorf("SaveEdit = %v, placed %t; the repository's file holds %q (%v); want the edit saved with the "+
			"secrets placed and kept elsewhere", err, placed, written, readErr)
	}
}

func TestSaveEditKeepsTheRepositoryFilesOwnCredentialAndAnInheritedOne(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, `{"forge": {"token": "forge-token-repo-5678"}}`)
	edited := read.Redacted()
	edited.Jira.Project = ossProject

	// Act
	_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

	// Assert
	saved := reread(t, files)
	if err != nil || saved.Forge.Token.Reveal() != "forge-token-repo-5678" || saved.Jira.Token.Reveal() != homeToken ||
		saved.Jira.Project != ossProject {
		t.Errorf("SaveEdit = %v, saved %+v; want both credentials kept beside the project changed",
			err, saved.Redacted())
	}
}

func TestSaveEditRemovesACredentialFromTheRepositoryFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, `{"forge": {"token": "forge-token-repo-5678"}}`)

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: read.Redacted(),
		Removed: []config.Credential{config.CredentialForgeToken},
	})

	// Assert
	if saved := reread(t, files); err != nil || saved.Forge.Token != "" {
		t.Errorf("SaveEdit = %v, forge token kept %t; want the repository's token removed", err, saved.Forge.Token != "")
	}
}

func TestSaveEditStillTakesATypedCredentialIntoTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Forge.Token = typedToken

	// Act
	_, _, err := config.SaveEdit(config.Edit{Files: files, Read: read, Over: revision, Edited: edited})

	// Assert
	if saved := reread(t, files); err != nil || saved.Forge.Token.Reveal() != typedToken {
		t.Errorf("SaveEdit = %v; want the typed forge token kept in the home file", err)
	}
}
