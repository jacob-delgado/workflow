// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

// A pair kept in a configuration file is written under the save locks every
// workflow's configuration save holds, so a save another workflow makes
// between the store's read and its write is waited for, never written over.

import (
	"errors"
	"testing"
	"testing/synctest"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// savingElsewhere is the save locks kept in a state directory of the test's
// own, with files' locks held as another workflow saving them would hold them
// until the test ends.
func savingElsewhere(t *testing.T, files config.Files) config.SaveLocks {
	t.Helper()

	locks := config.SaveLocksIn(t.TempDir())

	unlock, err := locks.Hold(files)
	if err != nil {
		t.Fatalf("holding the save locks: %v", err)
	}

	t.Cleanup(unlock)

	return locks
}

func TestAFileStoreWritesNoPairWhileAnotherWorkflowSavesTheFile(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files := homeHolding(t, false)
		locks := savingElsewhere(t, files)

		before, err := config.RevisionOfLayers(files)
		if err != nil {
			t.Fatalf("reading the revision: %v", err)
		}

		// Act
		err = slackauth.FileStore(locks, files).Save(t.Context(), pairExpiringIn(0))

		// Assert
		after, revErr := config.RevisionOfLayers(files)
		if !errors.Is(err, config.ErrSaveLocked) || revErr != nil || after != before {
			t.Errorf("Save = %v; the files are as they were: %t (%v); want %v and no pair written", err,
				after == before, revErr, config.ErrSaveLocked)
		}
	})
}

func TestAKeychainLoginNamesNoAppWhileAnotherWorkflowSavesTheHomeFile(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		files := homeAndRepository(t)
		store := slackauth.KeychainStore(keychainHolding(t), savingElsewhere(t, files), files)

		// Act
		err := store.KeepLogin(t.Context(), clientID, pairExpiringIn(0))

		// Assert
		_, loadErr := store.Load(t.Context())
		if !errors.Is(err, config.ErrSaveLocked) || !errors.Is(loadErr, slackauth.ErrNotLoggedIn) ||
			messagingIn(t, files.Home).ClientID != "" {
			t.Errorf("KeepLogin = %v, the keychain: %v, the home file names %q; want %v, nothing kept and no app "+
				"named", err, loadErr, messagingIn(t, files.Home).ClientID, config.ErrSaveLocked)
		}
	})
}
