// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

// A save from Settings writes the file before it hands anything to the
// keychain or spends a Slack refresh token, so a save the files refuse leaves
// both as they were, and one the keychain or Slack refuses takes the write
// back.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// errSlackRefused is Slack refusing the refresh token typed.
var errSlackRefused = errors.New("slack refused the refresh token")

// changeOnDisk is another writer changing the file at path, for a test to call
// while the save is under way.
func changeOnDisk(t *testing.T, path string) {
	t.Helper()

	err := os.WriteFile(path, []byte(`{"jira": {"project": "ELSE"}}`), config.FileMode)
	if err != nil {
		t.Errorf("changing %s: %v", path, err)
	}
}

func TestSaveEditSpendsNoSlackRefreshTokenWhenTheKeychainRefusesTheJiraToken(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token, edited.Messaging.RefreshToken = typedToken, typedRefresh
	placed := false

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(config.Config) error {
			placed = true

			return nil
		},
		KeepJiraToken: func(string, string) error { return errKeychainLocked },
	})

	// Assert
	if !errors.Is(err, config.ErrTokenNotKept) || placed {
		t.Errorf("SaveEdit = %v, placed %t; want the keychain's refusal and the typed refresh token unspent",
			err, placed)
	}
}

func TestSaveEditSpendsTheSlackRefreshTokenOnlyForASaveMade(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.RefreshToken = typedRefresh
	placed := false

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(config.Config) error {
			placed = true

			changeOnDisk(t, files.Home)

			return nil
		},
	})

	// Assert
	if (err == nil) != placed {
		t.Errorf("SaveEdit = %v, placed %t; want the typed refresh token spent exactly when the save is made",
			err, placed)
	}
}

func TestSaveEditKeepsTheJiraTokenOnlyForASaveMade(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken
	kept := keptTokens{}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		KeepJiraToken: func(service, secret string) error {
			changeOnDisk(t, files.Home)

			return kept.keep(service, secret)
		},
	})

	// Assert
	if (err == nil) != (len(kept) == 1) {
		t.Errorf("SaveEdit = %v, the keychain handed %d; want the keychain replaced exactly when the save is made",
			err, len(kept))
	}
}

func TestSaveEditTakesTheWriteBackWhenSlackRefusesTheTypedSecrets(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Messaging.RefreshToken, edited.Jira.Project = typedRefresh, ossProject

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		PlaceSlackCredentials: func(config.Config) error { return errSlackRefused },
	})

	// Assert
	after, afterErr := config.RevisionOfLayers(files)
	if !errors.Is(err, errSlackRefused) || afterErr != nil || after != revision {
		t.Errorf("SaveEdit = %v; the file is at its revision before the save: %t (%v); want Slack's refusal and "+
			"the file as it was", err, after == revision, afterErr)
	}
}

func TestSaveEditTakesTheWriteBackWhenTheKeychainRefusesTheJiraToken(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token, edited.Jira.Project = typedToken, ossProject

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		KeepJiraToken: func(string, string) error { return errKeychainLocked },
	})

	// Assert
	after, afterErr := config.RevisionOfLayers(files)
	if !errors.Is(err, config.ErrTokenNotKept) || afterErr != nil || after != revision {
		t.Errorf("SaveEdit = %v; the file is at its revision before the save: %t (%v); want the keychain's "+
			"refusal and the file as it was", err, after == revision, afterErr)
	}
}

func TestSaveEditTakesBackAWriteOfAFileThatWasNotThere(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: filepath.Join(t.TempDir(), config.FileName)}
	edited := config.Default()
	edited.Messaging = config.Messaging{Kind: config.KindSlack, ClientID: "1.2", RefreshToken: typedRefresh}

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: config.Default(), Edited: edited,
		PlaceSlackCredentials: func(config.Config) error { return errSlackRefused },
	})

	// Assert
	_, statErr := os.Stat(files.Home)
	if !errors.Is(err, errSlackRefused) || !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("SaveEdit = %v, the file %v; want Slack's refusal and no file", err, statErr)
	}
}

func TestSaveEditLeavesAFileChangedSinceItsWriteToWhoeverChangedIt(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		KeepJiraToken: func(string, string) error {
			changeOnDisk(t, files.Home)

			return errKeychainLocked
		},
	})

	// Assert
	if !errors.Is(err, errKeychainLocked) || !errors.Is(err, config.ErrChangedOnDisk) ||
		reread(t, files).Jira.Project != "ELSE" {
		t.Errorf("SaveEdit = %v; want the keychain's refusal, the write not taken back over the change, and "+
			"the change kept", err)
	}
}

func TestSaveEditLeavesAFileItCannotReadSinceItsWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	files, read, revision := readHome(t)
	edited := read.Redacted()
	edited.Jira.Token = typedToken

	// Act
	_, _, err := config.SaveEdit(config.Edit{
		Files: files, Read: read, Over: revision, Edited: edited,
		KeepJiraToken: func(string, string) error {
			err := os.Remove(files.Home)
			if err == nil {
				err = os.Mkdir(files.Home, 0o700)
			}

			return errors.Join(errKeychainLocked, err)
		},
	})

	// Assert
	info, statErr := os.Stat(files.Home)
	if !errors.Is(err, config.ErrChangedOnDisk) || statErr != nil || !info.IsDir() {
		t.Errorf("SaveEdit = %v, %v; want the write not taken back over what replaced the file", err, statErr)
	}
}
