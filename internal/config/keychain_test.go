// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"reflect"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// otherJiraURL is a Jira at another address than jiraURL.
const otherJiraURL = "https://jira.other.example"

// keychainHome is a home file reading its Jira token from the keychain, beside
// a token of its own and the sources a repository's file may not borrow.
const keychainHome = `{"jira": {"base_url": "` + jiraURL + `", "keychain": true,
  "token": "jira-token-home-1234", "token_command": "pass show jira", "token_env": "JIRA_TOKEN"}}`

func TestEachJiraAddressNamesAKeychainItemOfItsOwn(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		baseURL, want string
	}{
		"an address":      {baseURL: jiraURL, want: "workflow-jira " + jiraURL},
		"another address": {baseURL: otherJiraURL, want: "workflow-jira " + otherJiraURL},
		"an address with a path": {
			baseURL: "https://example.com/jira", want: "workflow-jira https://example.com/jira",
		},
		"the same address, slashed": {baseURL: jiraURL + "//", want: "workflow-jira " + jiraURL},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := config.Jira{BaseURL: tt.baseURL}.KeychainService()

			// Assert
			if got != tt.want {
				t.Errorf("KeychainService(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestARepositoryMovingJiraKeepsTheKeychainButNoHomeCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layeredOver(t, keychainHome, `{"jira": {"base_url": "`+otherJiraURL+`"}}`)

	// Act
	cfg, _, err := config.LoadLayersAt(files)
	// Assert
	if err != nil {
		t.Fatalf("LoadLayersAt = %v, want the layers read", err)
	}

	if got := credentialsOf(cfg); !reflect.DeepEqual(got, heldCredentials{}) {
		t.Errorf("the moved section holds %+v, want none of the home file's credentials", got)
	}

	if !cfg.Jira.Keychain || cfg.Jira.KeychainService() != "workflow-jira "+otherJiraURL {
		t.Errorf("keychain = %t naming %q, want the keychain read for the repository's own address",
			cfg.Jira.Keychain, cfg.Jira.KeychainService())
	}
}

func TestARepositoryFileMaySetTheKeychain(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layeredOver(t, `{"jira": {"base_url": "`+jiraURL+`"}}`,
		`{"jira": {"base_url": "`+otherJiraURL+`", "keychain": true}}`)

	// Act
	cfg, _, err := config.LoadLayersAt(files)

	// Assert
	if err != nil || !cfg.Jira.Keychain {
		t.Errorf("LoadLayersAt = keychain %t, %v; want the repository's keychain read, unrefused",
			cfg.Jira.Keychain, err)
	}
}

func TestTheKeychainIsATokenSource(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := config.Config{
		Jira:      config.Jira{BaseURL: jiraURL, Keychain: true},
		Messaging: config.Messaging{WebhookURL: webhookURL},
	}

	// Act
	missing := cfg.Missing()

	// Assert
	if len(missing) != 0 || cfg.Jira.AuthMode() != config.AuthBearer {
		t.Errorf("Missing() = %v and AuthMode() = %v, want nothing missing and a bearer token",
			missing, cfg.Jira.AuthMode())
	}
}
