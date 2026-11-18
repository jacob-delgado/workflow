// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

// Slack spends a refresh token as it answers, so the place the new pair goes
// is proved to take a write before a refresh is made: one that would not is
// ErrKeepRefused, with nothing spent, where a refusal after the refresh would
// lose the only pair that works.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// keptPair is the pair the keychain holds, as it holds it, due a refresh.
const keptPair = `{"client_secret":"` + clientSecret + `","refresh_token":"` + oldRefresh +
	`","access_token":"` + oldAccess + `","expires_at":""}`

// keychainRefusal is the status a locked keychain's security exits with.
const keychainRefusal = 51

// directoryAdvice is what a store refused for its keychain or its directory
// says to do, and fixTheFile what one whose file does not parse says instead.
const (
	directoryAdvice = "unlock the keychain or make the configuration file's directory writable"
	fixTheFile      = "fix the file"
)

// errProveRefused is a store that would not take a write.
var errProveRefused = errors.New("the store would not take a write")

// errProveRanOn is a prove that ran on after its asker left.
var errProveRanOn = errors.New("the prove ran on after its asker left")

// proveRunsOn is how long a prove waits for its asker's end before it calls
// itself run on: far past what a prove ending with its asker takes.
const proveRunsOn = 5 * time.Second

// probed is what a prove makes in the keychain: its probe stored, then
// deleted, and nothing else.
func probed() []string {
	return []string{"stored workflow-slack-probe", "deleted workflow-slack-probe"}
}

// agedDirectory sets dir's modification time a day back, and is that time,
// so a file made or removed in dir afterwards shows in its modification time.
func agedDirectory(t *testing.T, dir string) time.Time {
	t.Helper()

	aged := time.Now().Add(-24 * time.Hour).Truncate(time.Second)

	err := os.Chtimes(dir, aged, aged)
	if err != nil {
		t.Fatalf("aging %s: %v", dir, err)
	}

	return aged
}

// modifiedSince reports dir modified after aged: a file made or removed in it.
func modifiedSince(t *testing.T, dir string, aged time.Time) bool {
	t.Helper()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	return info.ModTime().After(aged)
}

// readOnlyDirectory makes dir one nothing can be written in, until the test
// ends; root writes in any directory, so the test is skipped under it.
func readOnlyDirectory(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root writes in any directory")
	}

	err := os.Chmod(dir, 0o500)
	if err != nil {
		t.Fatalf("making %s read-only: %v", dir, err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
}

func TestAKeychainThatReadsButWillNotStoreRefusesBeforeTheRefresh(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)
	held := &keychainByService{kept: map[string]string{slackauth.KeychainService: keptPair}, storing: keychainRefusal}
	source := slackauth.Source{
		Store: slackauth.KeychainStore(held.item(t), config.SaveLocks{}, config.Files{}), Refresher: refresherFor(server),
		Lock: slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock")), Now: testNow,
	}

	// Act
	_, err := source.Token(t.Context(), "")

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || len(seen()) != 0 {
		t.Errorf("Token = %v after %d refreshes; want %v and no refresh token spent",
			err, len(seen()), slackauth.ErrKeepRefused)
	}

	if held.kept[slackauth.KeychainService] != keptPair {
		t.Errorf("the keychain holds %q, want the pair it held", held.kept[slackauth.KeychainService])
	}
}

func TestARenewalWhoseProveRefusesSpendsNoRefreshToken(t *testing.T) {
	t.Parallel()

	// Arrange
	server, seen := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(0)}
	inner := held.store()
	refusing := slackauth.NewStore("memory", inner.Load, inner.Save,
		func(context.Context) error { return errProveRefused })
	source := slackauth.Source{
		Store: refusing, Refresher: refresherFor(server), Now: testNow,
		Lock: slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock")),
	}

	// Act
	_, err := source.Token(t.Context(), "")

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || !errors.Is(err, errProveRefused) ||
		!strings.Contains(err.Error(), "memory") || len(seen()) != 0 || held.saves != 0 {
		t.Errorf("Token = %v after %d refreshes and %d saves; want %v naming where, carrying why, "+
			"and nothing spent or kept", err, len(seen()), held.saves, slackauth.ErrKeepRefused)
	}
}

func TestAProveEndsWhenItsAskerLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	// A prove spends nothing, so it runs on the asker's context: a web
	// request that leaves while security hangs takes the prove with it.
	server, seen := fakeSlack(t, rotated)
	held := &memory{held: pairExpiringIn(0)}
	inner := held.store()
	asking, leave := context.WithCancel(t.Context())
	t.Cleanup(leave)

	hanging := slackauth.NewStore("memory", inner.Load, inner.Save, func(ctx context.Context) error {
		leave()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(proveRunsOn):
			return errProveRanOn
		}
	})
	source := slackauth.Source{
		Store: hanging, Refresher: refresherFor(server), Now: testNow,
		Lock: slackauth.FileLock(filepath.Join(t.TempDir(), "slack.lock")),
	}

	// Act
	_, err := source.Token(asking, "")

	// Assert
	if !errors.Is(err, context.Canceled) || len(seen()) != 0 {
		t.Errorf("Token = %v after %d refreshes; want the prove ended with its asker, nothing spent",
			err, len(seen()))
	}
}

func TestAKeychainProveLeavesThePairItHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainByService{kept: map[string]string{slackauth.KeychainService: keptPair}}
	store := slackauth.KeychainStore(held.item(t), config.SaveLocks{}, config.Files{})

	// Act
	err := store.Prove(t.Context())
	// Assert
	if err != nil || !slices.Equal(held.made(), probed()) {
		t.Fatalf("Prove = %v having made %q, want a keychain that stores proved by its probe alone",
			err, held.made())
	}

	if services := held.services(); len(services) != 1 || held.kept[slackauth.KeychainService] != keptPair {
		t.Errorf("the keychain keeps %q, want the pair alone, as it was, and no probe left", services)
	}
}

func TestAKeychainHoldingNoPairIsStillProven(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainByService{kept: map[string]string{}}
	files := config.Files{Home: configFile(t, `{}`)}
	store := slackauth.KeychainStore(held.item(t), config.SaveLocks{}, files)

	// Act
	err := store.ProveLogin(t.Context())

	// Assert
	if err != nil || len(held.services()) != 0 || !slices.Equal(held.made(), probed()) {
		t.Errorf("ProveLogin = %v having made %q, the keychain keeps %q; want it proved by its probe, "+
			"and nothing left in it", err, held.made(), held.services())
	}
}

func TestALoginAKeychainWillNotStoreForIsRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainByService{kept: map[string]string{}, storing: keychainRefusal}
	files := config.Files{Home: configFile(t, `{}`)}

	// Act
	err := slackauth.KeychainStore(held.item(t), config.SaveLocks{}, files).ProveLogin(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || !strings.Contains(err.Error(), "keychain") ||
		!strings.Contains(err.Error(), directoryAdvice) {
		t.Errorf("ProveLogin = %v, want %v naming the keychain, saying to %s", err, slackauth.ErrKeepRefused,
			directoryAdvice)
	}
}

func TestAProbeTheKeychainCannotDeleteRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainByService{kept: map[string]string{slackauth.KeychainService: keptPair}, deleting: keychainRefusal}

	// Act
	err := slackauth.KeychainStore(held.item(t), config.SaveLocks{}, config.Files{}).Prove(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || !errors.Is(err, keychain.ErrNotDeleted) {
		t.Errorf("Prove = %v, want %v carrying the probe not deleted", err, slackauth.ErrKeepRefused)
	}
}

func TestAFileStoreThatTakesAWriteIsProvenAndLeftAsItWas(t *testing.T) {
	t.Parallel()

	// Arrange
	contents := `{"messaging": {"kind": "slack", "client_id": "` + clientID + `", "channel": "#dev"}}`
	path := configFile(t, contents)
	aged := agedDirectory(t, filepath.Dir(path))

	// Act
	err := slackauth.FileStore(config.SaveLocks{}, config.Files{Home: path}).Prove(t.Context())

	// Assert
	after, readErr := os.ReadFile(path)
	if err != nil || readErr != nil || string(after) != contents {
		t.Errorf("Prove = %v; the file reads %q (%v); want it proved and unchanged", err, after, readErr)
	}

	if !modifiedSince(t, filepath.Dir(path), aged) {
		t.Error("the file's directory is as it was, want a probe made and removed beside the file")
	}
}

func TestAFileStoreInADirectoryNothingCanBeWrittenInRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	path := configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`"}}`)
	readOnlyDirectory(t, filepath.Dir(path))

	// Act
	err := slackauth.FileStore(config.SaveLocks{}, config.Files{Home: path}).Prove(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || !strings.Contains(err.Error(), path) ||
		!strings.Contains(err.Error(), directoryAdvice) {
		t.Errorf("Prove = %v, want %v naming %s, saying to %s", err, slackauth.ErrKeepRefused, path, directoryAdvice)
	}
}

func TestAFileStoreOverAFileItCannotParseRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	// A key no configuration has: the secrets read, but a save would refuse
	// the file, so the pair a refresh gave would never be kept.
	path := configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`"}, "unknown": true}`)

	// Act
	err := slackauth.FileStore(config.SaveLocks{}, config.Files{Home: path}).Prove(t.Context())

	// Assert
	said := err.Error()
	if !errors.Is(err, slackauth.ErrKeepRefused) || !errors.Is(err, config.ErrInvalid) ||
		!strings.Contains(said, fixTheFile) || strings.Contains(said, directoryAdvice) {
		t.Errorf("Prove = %v, want %v carrying the file refused, saying to %s and not to %s", err,
			slackauth.ErrKeepRefused, fixTheFile, directoryAdvice)
	}
}

func TestALoginProvesTheFileItNamesItsAppIn(t *testing.T) {
	t.Parallel()

	// Arrange
	held := &keychainByService{kept: map[string]string{}}
	home := configFile(t, `{}`)
	readOnlyDirectory(t, filepath.Dir(home))

	// Act
	err := slackauth.KeychainStore(held.item(t), config.SaveLocks{}, config.Files{Home: home}).ProveLogin(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrKeepRefused) || len(held.services()) != 0 {
		t.Errorf("ProveLogin = %v, the keychain keeps %q; want %v for the file, and the keychain untouched",
			err, held.services(), slackauth.ErrKeepRefused)
	}
}
