// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// homeConfig is the configuration in the home directory the layering tests
// start a repository's file over.
const homeConfig = `{"jira": {"base_url": "https://jira.example.com", "token": "jira-token-home-1234"},` +
	` "messaging": {"webhook_url": "https://hooks.slack.example/home"}}`

// homeAndRepository is a home directory holding homeConfig and a repository
// beside it, with nothing in the repository yet.
func homeAndRepository(t *testing.T) place {
	t.Helper()

	home := t.TempDir()
	writeFile(t, home, homeConfig)

	repo := t.TempDir()

	err := os.Mkdir(filepath.Join(repo, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}

	return place{dir: repo, home: home}
}

func TestConfigInitTemplateOverAHomeFileWritesAnEmptyLayer(t *testing.T) {
	// Arrange
	where := homeAndRepository(t)

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "config", "init", "--template")

	// Assert
	written, readErr := os.ReadFile(filepath.Join(where.dir, config.FileName))
	if err != nil || readErr != nil || strings.TrimSpace(string(written)) != "{}" {
		t.Fatalf("config init = %v; wrote %q (%v); want an empty layer", err, written, readErr)
	}

	if !strings.Contains(printed.stderr, filepath.Join(where.home, config.FileName)) {
		t.Errorf("config init said:\n%s\nwant it to name the home file the layer lies over", printed.stderr)
	}
}

func TestGuidedInitOverAHomeFileKeepsWhatWasLeftBlank(t *testing.T) {
	// Arrange
	where := homeAndRepository(t)

	// Act
	_, err := runStreamsAt(t, where, scripted([]string{""}, []string{""}), "config", "init")
	// Assert
	if err != nil {
		t.Fatalf("config init: %v", err)
	}

	cfg, err := config.Load(where.dir, where.home)
	if err != nil || cfg.Jira.Token != "jira-token-home-1234" || cfg.Messaging.WebhookURL == "" {
		t.Errorf("loaded %+v, %v; want the home file's Jira and webhook kept", cfg, err)
	}
}

func TestConfigShowNamesEveryFileInEffect(t *testing.T) {
	// Arrange
	where := homeAndRepository(t)
	writeFile(t, where.dir, `{"jira": {"project": "OSS"}}`)

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "config", "show")

	// Assert
	home, repo := filepath.Join(where.home, config.FileName), filepath.Join(where.dir, config.FileName)
	if err != nil || !strings.Contains(printed.stderr, home) || !strings.Contains(printed.stderr, repo) {
		t.Errorf("config show = %v, said:\n%s\nwant it to name %s and %s", err, printed.stderr, repo, home)
	}
}

func TestDoctorNamesEveryFileInEffect(t *testing.T) {
	// Arrange
	where := homeAndRepository(t)
	writeFile(t, where.dir, `{"jira": {"project": "OSS"}}`)

	// Act
	printed, _ := runStreamsAt(t, where, unusedPrompt(t), "doctor", "--json")

	// Assert
	var report struct {
		Configuration struct {
			Path  string   `json:"path"`
			Files []string `json:"files"`
		} `json:"configuration"`
	}

	err := json.Unmarshal([]byte(printed.stdout), &report)

	home, repo := filepath.Join(where.home, config.FileName), filepath.Join(where.dir, config.FileName)
	got := report.Configuration

	if err != nil || got.Path != repo || !slices.Equal(got.Files, []string{home, repo}) {
		t.Errorf("configuration = %+v, %v; want saving to %s, read from %s then %s", got, err, repo, home, repo)
	}
}
