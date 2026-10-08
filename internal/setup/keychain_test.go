// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// jiraItem is the keychain item the token for jiraAddress is kept in.
const jiraItem = "workflow-jira " + jiraAddress

// errKeychainLocked is a keychain that would not store.
var errKeychainLocked = errors.New("the keychain is locked")

// keychain is a fake keychain that keeps what it is handed, and the item it
// was kept under.
type keychain struct {
	service, stored string
}

// store keeps secret under service.
func (k *keychain) store(service, secret string) error {
	k.service, k.stored = service, secret

	return nil
}

// refusingKeychain is a keychain that will not store.
func refusingKeychain(string, string) error {
	return errKeychainLocked
}

func TestWriteKeepsTheTokenOutOfTheFileWhenTheKeychainIsChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = kept.store
	request := answered(setup.Home)
	request.Keychain = true

	// Act
	written, err := guide.Write(t.Context(), request)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	contents, err := os.ReadFile(written.Path)
	if err != nil {
		t.Fatalf("reading what was written: %v", err)
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: written.Path})
	if err != nil || strings.Contains(string(contents), typedToken) || !cfg.Jira.Keychain {
		t.Errorf("the file holds the token, or does not read the keychain (%v):\n%s", err, contents)
	}

	if kept.stored != typedToken || kept.service != jiraItem || !written.Keychain {
		t.Errorf("the keychain kept %q under %q (written %+v), want the typed token under %q",
			kept.stored, kept.service, written, jiraItem)
	}
}

func TestWriteKeepsTheTokenOfARepositoryFileInTheKeychainForItsAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = kept.store
	writeHomeFile(t, guide, `{"jira": {"base_url": "https://jira.home.example", "token": "home-token-9999"}}`)

	request := answered(setup.Repository)
	request.Keychain = true

	// Act
	written, err := guide.Write(t.Context(), request)

	// Assert
	contents, readErr := os.ReadFile(guide.Where.Path(setup.Repository))
	if err != nil || readErr != nil || !written.Keychain || strings.Contains(string(contents), typedToken) {
		t.Fatalf("Write = %+v, %v; the repository's file holds %q (%v); want it written, the token kept out",
			written, err, contents, readErr)
	}

	cfg, _, loadErr := config.LoadLayersAt(guide.Where.Layers(setup.Repository))
	if loadErr != nil || !cfg.Jira.Keychain || cfg.Jira.BaseURL != jiraAddress {
		t.Errorf("the layers read keychain %t for %q (%v); want the keychain read for the repository's address",
			cfg.Jira.Keychain, cfg.Jira.BaseURL, loadErr)
	}

	if kept.service != jiraItem || kept.stored != typedToken {
		t.Errorf("the keychain kept the token %t under %q, want it under %q",
			kept.stored == typedToken, kept.service, jiraItem)
	}
}

func TestWriteKeepsTheKeychainForARepositoryRootedAtHome(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()
	kept := &keychain{}
	guide := setup.Guide{Where: setup.Where{WorkDir: home, HomeDir: home}, Doer: acceptingJira(), StoreSecret: kept.store}
	request := answered(setup.Repository)
	request.Keychain = true

	// Act
	written, err := guide.Write(t.Context(), request)

	// Assert
	cfg, loadErr := config.Load(home, home)
	if err != nil || loadErr != nil || !cfg.Jira.Keychain || cfg.Jira.Token != "" || !written.Keychain {
		t.Errorf("Write = %+v, %v; loaded %+v (%v); want the home file reading the keychain",
			written, err, cfg.Jira, loadErr)
	}
}

func TestWriteKeepsTheTokenInAPrivateFileWhenTheKeychainIsDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = kept.store

	// Act
	written, err := guide.Write(t.Context(), answered(setup.Repository))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	cfg, _, err := config.LoadLayersAt(config.Files{Home: written.Path})
	if err != nil || cfg.Jira.Token.Reveal() != typedToken || kept.stored != "" {
		t.Errorf("wrote %+v (%v), keychain %q; want the token in the file alone", cfg.Jira, err, kept.stored)
	}

	info, err := os.Stat(written.Path)
	if err != nil || info.Mode().Perm() != config.FileMode {
		t.Errorf("the file's mode = %v (%v), want %v", info.Mode().Perm(), err, config.FileMode)
	}
}

func TestWriteRefusesTheKeychainWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	request := answered(setup.Home)
	request.Keychain = true

	// Act
	_, err := guide.Write(t.Context(), request)

	// Assert
	if !errors.Is(err, setup.ErrNoKeychain) {
		t.Errorf("Write with no keychain = %v, want %v", err, setup.ErrNoKeychain)
	}

	_, statErr := os.Stat(guide.Where.Path(setup.Home))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a refused write left a file: %v", statErr)
	}
}

func TestWriteReportsAKeychainThatWouldNotStore(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = refusingKeychain
	request := answered(setup.Home)
	request.Keychain = true

	// Act
	_, err := guide.Write(t.Context(), request)

	// Assert
	if !errors.Is(err, errKeychainLocked) {
		t.Errorf("Write = %v, want the keychain's refusal", err)
	}
}

func TestWriteIgnoresTheKeychainWithJiraLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	request := setup.Request{Place: setup.Home, Keychain: true}

	// Act
	written, err := guide.Write(t.Context(), request)

	// Assert
	if err != nil || written.Keychain {
		t.Errorf("Write = %+v, %v; want the file written with nothing for the keychain", written, err)
	}
}

func TestKeepRefusesAnEmptyToken(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}

	// Act
	_, err := setup.Keep(kept.store, config.Jira{BaseURL: jiraAddress})

	// Assert
	if !errors.Is(err, setup.ErrNoToken) || kept.stored != "" {
		t.Errorf("Keep = %v, keychain %q; want ErrNoToken and nothing stored", err, kept.stored)
	}
}

func TestKeepKeepsTheTokenInTheItemForItsAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}
	settings := config.Jira{BaseURL: "https://jira.other.example/", Token: typedToken, TokenCommand: "pass show jira"}

	// Act
	held, err := setup.Keep(kept.store, settings)

	// Assert
	want := config.Jira{BaseURL: settings.BaseURL, Keychain: true}
	if err != nil || !reflect.DeepEqual(held, want) {
		t.Errorf("Keep = keychain %t, a token %t, command %q, %v; want the keychain read alone",
			held.Keychain, held.Token != "", held.TokenCommand, err)
	}

	if kept.service != "workflow-jira https://jira.other.example" || kept.stored != typedToken {
		t.Errorf("the keychain kept %q under %q, want the token under the address's own item",
			kept.stored, kept.service)
	}
}

func TestWriteWithAnEmptyTokenLeavesTheKeychainAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	stored := false
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = func(string, string) error {
		stored = true

		return nil
	}
	request := setup.Request{
		Place: setup.Home, Answers: setup.Answers{Jira: config.Jira{BaseURL: jiraAddress}},
		Keychain: true,
	}

	// Act
	written, err := guide.Write(t.Context(), request)

	// Assert
	cfg, _, loadErr := config.LoadLayersAt(config.Files{Home: written.Path})
	if err != nil || stored || written.Keychain || written.Path != guide.Where.Path(setup.Home) || loadErr != nil ||
		cfg.Jira.BaseURL != jiraAddress || cfg.Jira.Token != "" || cfg.Jira.Keychain {
		t.Errorf("Write = %+v, %v, keychain called %t; wrote %+v (%v); want the home file written with Jira's "+
			"address and no token, and nothing for the keychain", written, err, stored, cfg.Redacted().Jira, loadErr)
	}
}

func TestWriteWhoseFileCannotBeMadeLeavesTheKeychainAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := &keychain{}
	guide := setup.Guide{
		Where: setup.Where{WorkDir: t.TempDir(), HomeDir: filepath.Join(t.TempDir(), "gone")},
		Doer:  acceptingJira(), StoreSecret: kept.store,
	}
	request := answered(setup.Home)
	request.Keychain = true

	// Act
	_, err := guide.Write(t.Context(), request)

	// Assert
	if err == nil || kept.stored != "" {
		t.Errorf("Write = %v, keychain %q; want the write refused before the keychain is touched", err, kept.stored)
	}
}

func TestWriteWhoseKeychainFailsLeavesNoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = refusingKeychain
	request := answered(setup.Home)
	request.Keychain = true

	// Act
	_, err := guide.Write(t.Context(), request)

	// Assert
	_, statErr := os.Lstat(guide.Where.Path(setup.Home))
	if !errors.Is(err, errKeychainLocked) || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Write = %v, file %v; want the keychain's refusal and no file left", err, statErr)
	}
}

// writeHomeFile writes contents as the home file guide lays a repository's
// file over.
func writeHomeFile(t *testing.T, guide setup.Guide, contents string) {
	t.Helper()

	err := os.WriteFile(guide.Where.Path(setup.Home), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}
}
