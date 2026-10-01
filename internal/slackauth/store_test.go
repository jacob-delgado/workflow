// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package slackauth_test

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/keychain"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/slackauth"
)

// inTheFile is what the configuration file's store says of where it keeps them.
const inTheFile = "file"

// configFile writes a configuration file holding contents and returns its path.
func configFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), config.FileName)

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing the configuration: %v", err)
	}

	return path
}

func TestTheFileStoreKeepsThePairAndEverythingElse(t *testing.T) {
	t.Parallel()

	// Arrange
	path := configFile(t, `{"jira": {"base_url": "https://jira.example.com"},`+
		` "messaging": {"kind": "slack", "client_id": "`+clientID+`", "channel": "#dev"}}`)
	store := slackauth.FileStore(config.Files{Repo: path})
	pair := pairExpiringIn(0)

	// Act
	err := store.Save(t.Context(), pair)
	// Assert
	if err != nil {
		t.Fatalf("Save = %v", err)
	}

	loaded, err := store.Load(t.Context())
	if err != nil || loaded != pair {
		t.Errorf("Load = %+v, %v; want the pair saved", loaded, err)
	}

	cfg, err := config.LoadFile(path)
	if err != nil || cfg.Jira.BaseURL != "https://jira.example.com" || cfg.Messaging.Channel != "#dev" ||
		cfg.Messaging.ClientID != clientID {
		t.Errorf("the file reads %+v, %v; want the rest of it as it was", cfg, err)
	}
}

func TestAFileStoreWithNothingSavedSaysToLogIn(t *testing.T) {
	t.Parallel()

	// Arrange
	path := configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`"}}`)

	// Act
	_, err := slackauth.FileStore(config.Files{Repo: path}).Load(t.Context())

	// Assert
	if !errors.Is(err, slackauth.ErrNotLoggedIn) {
		t.Errorf("Load = %v, want %v", err, slackauth.ErrNotLoggedIn)
	}
}

// keychainHolding is a macOS keychain item whose security keeps what it is
// last handed and reads it back.
func keychainHolding(t *testing.T) keychain.Item {
	t.Helper()

	var kept string

	run := func(_ context.Context, program proc.Command, input []byte) ([]byte, error) {
		if program.Args[0] == "-i" {
			line := string(input)
			start := strings.Index(line, ` -w "`) + len(` -w "`)
			kept = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(strings.TrimSuffix(line[start:], "\"\n"))

			return nil, nil
		}

		return []byte(kept + "\n"), nil
	}

	item, ok := keychain.Open("darwin", "workflow-slack", run,
		func() (*user.User, error) { return &user.User{Username: "jacob"}, nil }, func(string) string { return "" })
	if !ok {
		t.Fatal("no keychain item on darwin")
	}

	return item
}

func TestTheKeychainStoreKeepsThePair(t *testing.T) {
	t.Parallel()

	// Arrange
	store := slackauth.KeychainStore(keychainHolding(t))
	pair := pairExpiringIn(0)

	// Act
	err := store.Save(t.Context(), pair)
	// Assert
	if err != nil {
		t.Fatalf("Save = %v", err)
	}

	loaded, err := store.Load(t.Context())
	if err != nil || loaded != pair {
		t.Errorf("Load = %+v, %v; want the pair saved", loaded, err)
	}
}

func TestChooseKeepsThePairWhereItBelongs(t *testing.T) {
	t.Parallel()

	item := keychainHolding(t)
	cases := map[string]struct {
		goos      string
		messaging config.Messaging
		want      string
	}{
		"macOS keeps it in the keychain": {
			goos: "darwin", messaging: config.Messaging{ClientID: clientID}, want: "keychain",
		},
		"macOS keeps it in a file that already holds it": {
			goos: "darwin", messaging: config.Messaging{ClientID: clientID, RefreshToken: oldRefresh}, want: inTheFile,
		},
		"linux keeps it in the file": {goos: "linux", messaging: config.Messaging{ClientID: clientID}, want: inTheFile},
		"windows keeps it in the file": {
			goos: "windows", messaging: config.Messaging{ClientID: clientID}, want: inTheFile,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Config{Messaging: tt.messaging, Path: "/home/example/.workflow.json"}

			// Act
			store := slackauth.Choose(tt.goos, cfg, item)

			// Assert
			if got := store.Where(); !strings.Contains(got, tt.want) {
				t.Errorf("Choose kept it in %q, want the %s", got, tt.want)
			}
		})
	}
}

func TestAKeychainEntryMissingItsSecretsSaysToLogIn(t *testing.T) {
	t.Parallel()

	for name, entry := range map[string]string{
		"empty":            `{}`,
		"null":             `null`,
		"no client secret": `{"refresh_token":"xoxe-1-x"}`,
		"no refresh token": `{"client_secret":"s"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			item := keychainHolding(t)

			err := item.Store(t.Context(), entry)
			if err != nil {
				t.Fatalf("Store = %v", err)
			}

			// Act
			_, err = slackauth.KeychainStore(item).Load(t.Context())

			// Assert
			if !errors.Is(err, slackauth.ErrNotLoggedIn) {
				t.Errorf("Load = %v, want ErrNotLoggedIn", err)
			}
		})
	}
}

// homeHolding is a home file whose Slack app is named, holding the user
// token's secrets when held says so, beneath a repository file setting only a
// Jira project.
func homeHolding(t *testing.T, held bool) config.Files {
	t.Helper()

	secrets := ""
	if held {
		secrets = `, "client_secret": "` + clientSecret + `", "refresh_token": "` + oldRefresh + `"`
	}

	return config.Files{
		Home: configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`", "channel": "#dev"`+secrets+`}}`),
		Repo: configFile(t, `{"jira": {"project": "OSS"}}`),
	}
}

func TestTheUserTokenStaysInTheHomeFileBeneathARepositoryFile(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]bool{"a refreshed pair": true, "a first login": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := homeHolding(t, held)

			// Act
			err := slackauth.FileStore(files).Save(t.Context(), pairExpiringIn(0))

			// Assert
			home, homeErr := os.ReadFile(files.Home)
			repo, repoErr := os.ReadFile(files.Repo)

			if err != nil || homeErr != nil || repoErr != nil {
				t.Fatalf("Save = %v; reading the files: %v, %v", err, homeErr, repoErr)
			}

			if !strings.Contains(string(home), oldRefresh) || strings.Contains(string(repo), oldRefresh) {
				t.Errorf("home holds %s\nrepository holds %s\nwant the token at home alone", home, repo)
			}
		})
	}
}

func TestSecretsTheRepositoryFileHoldsStayThereAndItStaysAnOverlay(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{
		Home: configFile(t, `{"messaging": {"kind": "slack", "client_id": "`+clientID+`", "channel": "#dev"}}`),
		Repo: configFile(t, `{"messaging": {"client_secret": "`+clientSecret+`", "refresh_token": "old-refresh"}}`),
	}

	// Act
	err := slackauth.FileStore(files).Save(t.Context(), pairExpiringIn(0))

	// Assert
	repo, readErr := os.ReadFile(files.Repo)
	if err != nil || readErr != nil {
		t.Fatalf("Save = %v; reading the repository file: %v", err, readErr)
	}

	if !strings.Contains(string(repo), oldRefresh) || strings.Contains(string(repo), "timing") {
		t.Errorf("the repository file holds %s; want the new pair in it and nothing it inherits", repo)
	}
}
