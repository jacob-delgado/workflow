// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

// A Jira token typed into the web's Settings, where a keychain is wired, is
// kept in the keychain item for its address, so a repository pointing Jira at
// another address gets a token of its own and its file holds none.

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// The addresses and tokens a repository moving Jira is saved with.
const (
	homeJiraAddress   = "https://jira.example.com"
	movedJiraAddress  = "https://jira.other.example"
	homeFileJiraToken = "home-jira-token-1111"
	typedForMovedJira = "typed-jira-token-2222"
)

// errKeychainRefused is a keychain that would not keep a token.
var errKeychainRefused = errors.New("the keychain is locked")

// movedJiraServed serves a repository whose file points Jira at another
// address than the home file's, keeping a typed token through keep.
func movedJiraServed(t *testing.T, keep func(service, secret string) error) (http.Handler, config.Config) {
	t.Helper()

	home := filepath.Join(t.TempDir(), config.FileName)
	repo := filepath.Join(t.TempDir(), config.FileName)

	for path, contents := range map[string]string{
		home: `{"jira": {"base_url": "` + homeJiraAddress + `", "token": "` + homeFileJiraToken + `"}}`,
		repo: `{"jira": {"base_url": "` + movedJiraAddress + `"}}`,
	} {
		err := os.WriteFile(path, []byte(contents), config.FileMode)
		if err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	cfg, _, err := config.LoadLayersAt(config.Files{Home: home, Repo: repo})
	if err != nil {
		t.Fatalf("reading the layers: %v", err)
	}

	return serveWith(t, webserver.Deps{KeepJiraToken: keep}, cfg, webserver.Info{Version: testVersion}), cfg
}

// typingJiraToken is cfg with typedForMovedJira typed into its token.
func typingJiraToken(cfg config.Config) config.Config {
	cfg = cfg.Redacted()
	cfg.Jira.Token = typedForMovedJira

	return cfg
}

func TestATokenTypedForAMovedJiraIsKeptInTheKeychainForItsAddress(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := map[string]string{}
	handler, cfg := movedJiraServed(t, func(service, secret string) error {
		kept[service] = secret

		return nil
	})

	// Act
	saved := putConfig(t, handler, marshal(t, typingJiraToken(cfg)))

	// Assert
	if saved.Code != http.StatusOK || strings.Contains(saved.Body.String(), typedForMovedJira) {
		t.Fatalf("status = %d, want 200 carrying no token: %s", saved.Code, saved.Body.String())
	}

	written, readErr := os.ReadFile(cfg.Files.Repo)
	if readErr != nil || strings.Contains(string(written), typedForMovedJira) ||
		!strings.Contains(string(written), `"keychain": true`) {
		t.Errorf("the repository's file holds %q (%v); want it reading the keychain, the token kept out",
			written, readErr)
	}

	if len(kept) != 1 || kept["workflow-jira "+movedJiraAddress] != typedForMovedJira {
		t.Errorf("the keychain was handed %d items, the token for the moved address %t; want that alone",
			len(kept), kept["workflow-jira "+movedJiraAddress] == typedForMovedJira)
	}
}

func TestATokenTheKeychainWillNotKeepIsRefusedWithoutIt(t *testing.T) {
	t.Parallel()

	// Arrange
	handler, cfg := movedJiraServed(t, func(string, string) error { return errKeychainRefused })

	// Act
	saved := putConfig(t, handler, marshal(t, typingJiraToken(cfg)))

	// Assert
	if saved.Code != http.StatusUnprocessableEntity || strings.Contains(saved.Body.String(), typedForMovedJira) ||
		!strings.Contains(saved.Body.String(), "keychain") {
		t.Errorf("status = %d: %s; want 422 saying the keychain kept no token, naming none",
			saved.Code, saved.Body.String())
	}

	if written, _ := os.ReadFile(cfg.Files.Repo); strings.Contains(string(written), "keychain") {
		t.Errorf("the repository's file holds %q; want it as it was", written)
	}
}
