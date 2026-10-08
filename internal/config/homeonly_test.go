// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// jiraTokenCommand and jiraTokenVariable are a token_command and a token_env
// the home file sets.
const (
	jiraTokenCommand  = "pass show jira"
	jiraTokenVariable = "JIRA_TOKEN"
)

// homeOnlySettings are, by key, a file setting each thing only the home file
// may: a program to run, or an environment variable to read.
func homeOnlySettings() map[string]string {
	return map[string]string{
		"jira.token_command":  `{"jira": {"base_url": "https://jira.example.com", "token_command": "sh evil.sh"}}`,
		"jira.token_env":      `{"jira": {"base_url": "https://jira.example.com", "token_env": "AWS_SECRET_ACCESS_KEY"}}`,
		"taskwarrior.program": `{"taskwarrior": {"program": "/opt/evil/task"}}`,
	}
}

func TestARepositoryFileSettingAHomeOnlyKeyIsRefused(t *testing.T) {
	t.Parallel()

	for key, repo := range homeOnlySettings() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := layeredOver(t, `{}`, repo)

			// Act
			_, _, err := config.LoadLayersAt(files)

			// Assert
			if !errors.Is(err, config.ErrHomeOnly) || !errors.Is(err, config.ErrInvalid) ||
				!strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), files.Repo) {
				t.Errorf("LoadLayersAt = %v; want ErrHomeOnly naming %s and %s", err, key, files.Repo)
			}
		})
	}
}

func TestARepositoryFileStandingAloneMaySetNoHomeOnlyKey(t *testing.T) {
	t.Parallel()

	// Arrange
	repoDir := t.TempDir()

	err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}

	write(t, repoDir, homeOnlySettings()["jira.token_command"])

	// Act
	_, err = config.Load(repoDir, t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrHomeOnly) {
		t.Errorf("Load = %v; want a lone repository file's token_command refused with ErrHomeOnly", err)
	}
}

func TestTheHomeFileMaySetEveryHomeOnlyKey(t *testing.T) {
	t.Parallel()

	for key, home := range homeOnlySettings() {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := layeredOver(t, home, `{"jira": {"project": "OSS"}}`)

			// Act
			_, _, err := config.LoadLayersAt(files)
			// Assert
			if err != nil {
				t.Errorf("LoadLayersAt = %v; want the home file's %s taken", err, key)
			}
		})
	}
}

func TestARepositoryFileMayLeaveAHomeOnlyKeyEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layeredOver(t, tokenSourcesHome, `{"jira": {"token_command": "", "token_env": ""},
		"taskwarrior": {"program": ""}}`)

	// Act
	cfg, _, err := config.LoadLayersAt(files)

	// Assert
	if err != nil || cfg.Jira.TokenCommand != "" || cfg.Jira.TokenEnv != "" {
		t.Errorf("LoadLayersAt = %+v, %v; want the empty values to clear the home file's", cfg.Jira, err)
	}
}

func TestASaveRefusesToWriteAHomeOnlyKeyIntoTheRepositoryFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layeredOver(t, `{}`, `{"jira": {"project": "OSS"}}`)

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	cfg.Taskwarrior.Program = "/opt/homebrew/bin/task"

	// Act
	_, err = config.SaveLayers(files, cfg, over)

	// Assert
	written, readErr := os.ReadFile(files.Repo)
	if !errors.Is(err, config.ErrHomeOnly) || readErr != nil || strings.Contains(string(written), "homebrew") {
		t.Errorf("SaveLayers = %v; repository file %q (%v); want ErrHomeOnly and the file unchanged",
			err, written, readErr)
	}
}

func TestASaveWritesAHomeOnlyKeyIntoTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Home: write(t, t.TempDir(), `{}`)}

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	cfg.Jira.TokenCommand = jiraTokenCommand

	// Act
	_, err = config.SaveLayers(files, cfg, over)

	// Assert
	reloaded, _, loadErr := config.LoadLayersAt(files)
	if err != nil || loadErr != nil || reloaded.Jira.TokenCommand != jiraTokenCommand {
		t.Errorf("SaveLayers = %v, reloaded %+v (%v); want the home file to keep the command",
			err, reloaded.Jira, loadErr)
	}
}

func TestLayersAreTheFilesAConfigurationWasReadFrom(t *testing.T) {
	t.Parallel()

	read := config.Files{Home: "/home/u/.workflow.json", Repo: "/src/api/.workflow.json"}

	cases := map[string]struct {
		cfg  config.Config
		want config.Files
	}{
		"read from both files": {cfg: config.Config{Files: read, Path: read.Repo}, want: read},
		"built over one file":  {cfg: config.Config{Path: read.Home}, want: config.Files{Home: read.Home}},
		"built over none":      {cfg: config.Default(), want: config.Files{}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.cfg.Layers(); got != tt.want {
				t.Errorf("Layers = %+v, want %+v", got, tt.want)
			}
		})
	}
}
