// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

// A Jira token typed into Settings where a keychain is wired is kept in the
// keychain item for the address it is for, and the file saved reads it from
// there: so a repository that points Jira at another address gets a token of
// its own without one ever landing in its file.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errKeychainLocked is a keychain that would not keep what it was handed.
var errKeychainLocked = errors.New("the keychain is locked")

// keptTokens is a fake keychain: what it was handed, by the item named.
type keptTokens map[string]string

// keep keeps secret under service.
func (k keptTokens) keep(service, secret string) error {
	k[service] = secret

	return nil
}

// otherAddressRepo is a repository's file pointing Jira at another address.
const otherAddressRepo = `{"jira": {"base_url": "` + otherJiraURL + `"}}`

func TestSettingsKeepsATypedTokenInTheKeychainItemForItsAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, otherAddressRepo)
	edited := read.Redacted()
	edited.Jira.Token = typedToken
	kept := keptTokens{}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: kept.keep,
	})

	// Assert
	written, readErr := os.ReadFile(files.Repo)
	if err != nil || readErr != nil || strings.Contains(string(written), typedToken) {
		t.Fatalf("SaveEdit = %v; the repository's file holds %q (%v); want it saved without the token",
			err, written, readErr)
	}

	if saved := reread(t, files); !saved.Jira.Keychain || saved.Jira.Token != "" {
		t.Errorf("the saved files read keychain %t and hold a token %t; want the keychain read alone",
			saved.Jira.Keychain, saved.Jira.Token != "")
	}

	if len(kept) != 1 || kept["workflow-jira "+otherJiraURL] != typedToken {
		t.Errorf("the keychain kept %d items, the typed token for the repository's address %t; want that alone",
			len(kept), kept["workflow-jira "+otherJiraURL] == typedToken)
	}
}

func TestSettingsMovesATypedTokenOutOfTheHomeFileIntoTheKeychain(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken
	kept := keptTokens{}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: kept.keep,
	})

	// Assert
	saved := reread(t, files)
	if err != nil || saved.Jira.Token != "" || !saved.Jira.Keychain || kept["workflow-jira "+jiraURL] != typedToken {
		t.Errorf("SaveEdit = %v; the home file holds a token %t, reads the keychain %t; want the typed token "+
			"in the keychain alone", err, saved.Jira.Token != "", saved.Jira.Keychain)
	}
}

func TestSettingsKeepsAStoredTokenLeftMaskedWhereItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Project = ossProject
	kept := keptTokens{}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: kept.keep,
	})

	// Assert
	if saved := reread(t, files); err != nil || saved.Jira.Token.Reveal() != homeToken || len(kept) != 0 {
		t.Errorf("SaveEdit = %v; the token kept in the file %t, the keychain handed %d; want it left in the file",
			err, saved.Jira.Token.Reveal() == homeToken, len(kept))
	}
}

func TestSettingsTouchesNoKeychainWhenAnotherCredentialIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readLayered(t, otherAddressRepo)
	edited := read.Redacted()
	edited.Jira.Token, edited.Forge.Token = typedToken, typedToken
	kept := keptTokens{}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: kept.keep,
	})

	// Assert
	if !errors.Is(err, config.ErrCredentialInRepository) || len(kept) != 0 {
		t.Errorf("SaveEdit = %v, the keychain handed %d; want the forge token refused before the keychain is touched",
			err, len(kept))
	}
}

func TestSettingsTouchesNoKeychainOverFilesChangedSinceTheRead(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken
	kept := keptTokens{}

	err := os.WriteFile(files.Home, []byte(`{"jira": {"project": "ELSE"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("changing the file: %v", err)
	}

	// Act
	_, _, err = config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: kept.keep,
	})

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) || len(kept) != 0 {
		t.Errorf("SaveEdit = %v, the keychain handed %d; want ErrChangedOnDisk and the keychain untouched",
			err, len(kept))
	}
}

func TestSettingsWritesNothingWhenTheKeychainKeepsNoToken(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken
	refusing := func(string, string) error { return errKeychainLocked }

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited, KeepJiraToken: refusing,
	})

	// Assert
	saved := reread(t, files)
	if !errors.Is(err, config.ErrTokenNotKept) || !errors.Is(err, errKeychainLocked) ||
		strings.Contains(err.Error(), typedToken) || saved.Jira.Token.Reveal() != homeToken {
		t.Errorf("SaveEdit = %v; the home file still holds its token %t; want ErrTokenNotKept naming no token, "+
			"nothing written", err, saved.Jira.Token.Reveal() == homeToken)
	}
}
