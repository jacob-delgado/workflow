// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

// A login keeps the app a user token is for beside its secrets: in the file
// that keeps them, or, where the keychain keeps them, in the home file every
// directory reads, as every directory reads the keychain.

import (
	"context"
	"errors"
	"os/user"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// errSecurityRefused is security refusing to keep anything.
var errSecurityRefused = errors.New("security refused")

// homeAndRepository are a home file holding nothing and a repository's file
// over it naming only a channel.
func homeAndRepository(t *testing.T) config.Files {
	t.Helper()

	return config.Files{
		Home: configFile(t, `{}`),
		Repo: configFile(t, `{"messaging": {"channel": "#dev"}}`),
	}
}

// refusingKeychain is a macOS keychain item whose security refuses to keep
// anything.
func refusingKeychain(t *testing.T) keychain.Item {
	t.Helper()

	refusing := func(context.Context, proc.Command, []byte) ([]byte, error) { return nil, errSecurityRefused }

	item, ok := keychain.Open("darwin", slackauth.KeychainService, refusing,
		func() (*user.User, error) { return &user.User{Username: "jacob"}, nil }, func(string) string { return "" })
	if !ok {
		t.Fatal("no keychain item on darwin")
	}

	return item
}

// messagingIn is the messaging block the one file at path holds.
func messagingIn(t *testing.T, path string) config.Messaging {
	t.Helper()

	cfg, _, err := config.LoadLayersAt(config.Files{Home: path})
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return cfg.Messaging
}

func TestAFileLoginNamesTheAppBesideThePairItKeeps(t *testing.T) {
	t.Parallel()

	// Arrange
	files := homeAndRepository(t)
	pair := pairExpiringIn(0)

	// Act
	err := slackauth.FileStore(files).KeepLogin(t.Context(), clientID, pair)

	// Assert
	home, repository := messagingIn(t, files.Home), messagingIn(t, files.Repo)
	if err != nil || home.ClientID != clientID || home.Kind != config.KindSlack ||
		home.RefreshToken != pair.RefreshToken || repository.ClientID != "" || repository.HoldsUserTokenSecrets() {
		t.Errorf("KeepLogin = %v; the home file names %q (%s) holding the pair %t, the repository's names %q; "+
			"want the app and the pair together at home, and the repository's file as it was", err, home.ClientID,
			home.Kind, home.RefreshToken == pair.RefreshToken, repository.ClientID)
	}
}

func TestAKeychainLoginNamesTheAppInTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files := homeAndRepository(t)
	store := slackauth.KeychainStore(keychainHolding(t), files)
	pair := pairExpiringIn(0)

	// Act
	err := store.KeepLogin(t.Context(), clientID, pair)

	// Assert
	kept, loadErr := store.Load(t.Context())
	home := messagingIn(t, files.Home)

	if err != nil || loadErr != nil || kept != pair || home.ClientID != clientID || home.HoldsUserTokenSecrets() {
		t.Errorf("KeepLogin = %v, the keychain holds the pair %t (%v), the home file names %q holding secrets %t; "+
			"want the pair in the keychain and the app alone at home", err, kept == pair, loadErr, home.ClientID,
			home.HoldsUserTokenSecrets())
	}
}

func TestAKeychainLoginTheKeychainRefusesNamesNoApp(t *testing.T) {
	t.Parallel()

	// Arrange
	files := homeAndRepository(t)
	item := refusingKeychain(t)

	// Act
	err := slackauth.KeychainStore(item, files).KeepLogin(t.Context(), clientID, pairExpiringIn(0))

	// Assert
	if !errors.Is(err, slackauth.ErrNotKept) || messagingIn(t, files.Home).ClientID != "" {
		t.Errorf("KeepLogin = %v, the home file names %q; want ErrNotKept and no app named",
			err, messagingIn(t, files.Home).ClientID)
	}
}

func TestALoginOverAStoreOfItsOwnKeepsThePair(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &memory{}
	pair := pairExpiringIn(0)

	// Act
	err := held.store().KeepLogin(t.Context(), clientID, pair)

	// Assert
	if err != nil || held.held != pair {
		t.Errorf("KeepLogin = %v, holding %+v; want the pair kept", err, held.held)
	}
}

func TestAKeychainLoginThatCannotNameTheAppKeepsNoPair(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: configFile(t, `{"messaging": `)}
	store := slackauth.KeychainStore(keychainHolding(t), files)

	// Act
	err := store.KeepLogin(t.Context(), clientID, pairExpiringIn(0))

	// Assert
	_, loadErr := store.Load(t.Context())
	if !errors.Is(err, slackauth.ErrNotKept) || !errors.Is(loadErr, slackauth.ErrNotLoggedIn) {
		t.Errorf("KeepLogin = %v, then the keychain reads %v; want ErrNotKept and no pair kept without its app",
			err, loadErr)
	}
}

func TestAKeychainLoginTheKeychainRefusesLeavesTheAppNamedBefore(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: configFile(t, `{"messaging": {"kind": "slack", "client_id": "0000.1111"}}`)}
	item := refusingKeychain(t)

	// Act
	err := slackauth.KeychainStore(item, files).KeepLogin(t.Context(), clientID, pairExpiringIn(0))

	// Assert
	if home := messagingIn(t, files.Home); !errors.Is(err, slackauth.ErrNotKept) || home.ClientID != "0000.1111" {
		t.Errorf("KeepLogin = %v, the home file names %q; want ErrNotKept and the app named before", err, home.ClientID)
	}
}
