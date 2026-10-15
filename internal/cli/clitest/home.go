// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// homeConfig is the configuration in the home directory the layering tests
// start a repository's file over.
const homeConfig = `{"jira": {"base_url": "https://jira.example.com", "token": "jira-token-home-1234"},` +
	` "messaging": {"webhook_url": "https://hooks.slack.example/home"}}`

// markerMode is the mode of the .git directory that marks a repository's root.
const markerMode os.FileMode = 0o700

// HomeAndRepository is a home directory whose file sets a Jira with its token
// and a webhook, and a repository beside it, with nothing in the repository
// yet.
func HomeAndRepository(t *testing.T) Place {
	t.Helper()

	home := t.TempDir()
	WriteFile(t, home, homeConfig)

	repo := t.TempDir()

	err := os.Mkdir(filepath.Join(repo, ".git"), markerMode)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}

	return Place{Dir: repo, Home: home}
}

// BreakFile is what writes what no configuration parses over the file at
// path, for a test to call at the moment the file should stop parsing.
func BreakFile(t *testing.T, path string) func() {
	t.Helper()

	return func() {
		err := os.WriteFile(path, []byte("{"), config.FileMode)
		if err != nil {
			t.Errorf("breaking %s: %v", path, err)
		}
	}
}
