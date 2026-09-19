// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// The working directory's file REPLACES the home one. If the two ever merged,
// the home file's Slack token would leak into this result.
func TestLoadUsesTheWorkingDirectoryFileInsteadOfHomes(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	homeDir := t.TempDir()

	wantPath := write(t, workDir, `{"jira": {"base_url": "https://work.example.com"}}`)
	write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want %s", cfg.Path, wantPath)
	}

	if cfg.Jira.BaseURL != "https://work.example.com" {
		t.Errorf("jira.base_url = %q, want the working directory's value", cfg.Jira.BaseURL)
	}

	if cfg.Slack.Token != "" {
		t.Errorf("slack.token = %q, want empty: the home file must not merge in", cfg.Slack.Token)
	}
}

func TestLoadFallsBackToHome(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	homeDir := t.TempDir()
	wantPath := write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want %s", cfg.Path, wantPath)
	}

	if cfg.Slack.Channel != devChannel {
		t.Errorf("slack.channel = %q, want #dev", cfg.Slack.Channel)
	}
}

func TestLoadFindsTheRepositoryConfigFromASubdirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository whose config sits at its root, with work happening in a
	// subdirectory below it. The home directory holds a different file.
	repoRoot := t.TempDir()

	err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repo marker: %v", err)
	}

	wantPath := write(t, repoRoot, `{"jira": {"base_url": "https://repo.example.com"}}`)

	subDir := filepath.Join(repoRoot, "internal", "deep")

	err = os.MkdirAll(subDir, 0o700)
	if err != nil {
		t.Fatalf("making the subdirectory: %v", err)
	}

	homeDir := t.TempDir()
	write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(subDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	// The repository's own file is found, not skipped for the one at home.
	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want the repository's own %s", cfg.Path, wantPath)
	}

	if cfg.Jira.BaseURL != "https://repo.example.com" {
		t.Errorf("jira.base_url = %q, want the repository config's value", cfg.Jira.BaseURL)
	}
}

func TestLoadReportsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	_, err := config.Load(t.TempDir(), t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestLoadRejectsMalformedAndUnknownKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"malformed JSON": `{"jira": `,
		// A misspelled key that loaded silently would look exactly like a
		// credential the user never set.
		"unknown key": `{"jiraa": {"token": "x"}}`,
		"unknown nested key": `{"jira": {"base_url": "https://jira.example.com",` +
			` "tokenn": "x"}}`,
		"a request timeout that is not a duration": `{"timing": {"request_timeout": "fast"}}`,
		"a CI interval that is not positive":       `{"timing": {"ci_interval": "0s"}}`,
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, contents)

			// Act
			_, err := config.Load(workDir, t.TempDir())

			// Assert
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestDiscoverIgnoresAnEmptyDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	homeDir := t.TempDir()
	wantPath := write(t, homeDir, completeConfig)

	// Act
	// A machine with no home directory hands Load an empty string rather than a
	// path. That must skip the entry, not turn into a lookup of "/.workflow.json".
	got, err := config.Discover("", homeDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Assert
	if got != wantPath {
		t.Errorf("Discover = %s, want %s", got, wantPath)
	}
}

func TestLoadFileReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	// Act
	_, err := config.LoadFile(filepath.Join(t.TempDir(), "does-not-exist.json"))

	// Assert
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestRepoRootFindsTheEnclosingRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	// A subdirectory below a repository whose root holds .git.
	repoRoot := t.TempDir()

	err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repo marker: %v", err)
	}

	subDir := filepath.Join(repoRoot, "internal", "deep")

	err = os.MkdirAll(subDir, 0o700)
	if err != nil {
		t.Fatalf("making the subdirectory: %v", err)
	}

	// Act
	got := config.RepoRoot(subDir)

	// Assert
	// A config written here is found from any subdirectory below it.
	if got != repoRoot {
		t.Errorf("RepoRoot(%q) = %q, want the repository root %q", subDir, got, repoRoot)
	}
}

func TestRepoRootOutsideARepositoryIsTheDirectoryItself(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	got := config.RepoRoot(dir)

	// Assert
	if got != dir {
		t.Errorf("RepoRoot(%q) = %q, want the directory unchanged", dir, got)
	}
}
