// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// homeFile is a home configuration the layering tests put a repository's file
// over: a complete Jira, a Slack webhook, a list of views and the ASCII marks.
const homeFile = `{
  "jira": {"base_url": "https://jira.example.com", "token": "jira-token-home-1234", "project": "PROJ",
    "views": [{"name": "Mine", "jql": "assignee = currentUser()"}, {"name": "Team", "jql": "project = PROJ"}]},
  "messaging": {"webhook_url": "https://hooks.slack.example/home", "channel": "#dev"},
  "ui": {"ascii": true}
}`

// ossProject is the Jira project the repository's file sets over the home
// file's.
const ossProject = "OSS"

// layered writes homeFile and a repository file holding repo, the repository's
// in a directory marked as a repository root, and returns where each went.
func layered(t *testing.T, repo string) config.Files {
	t.Helper()

	repoDir := t.TempDir()

	err := os.Mkdir(filepath.Join(repoDir, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}

	return config.Files{Home: write(t, t.TempDir(), homeFile), Repo: write(t, repoDir, repo)}
}

// loadedFrom loads the layers files names, from the repository's directory.
func loadedFrom(t *testing.T, files config.Files) (config.Config, error) {
	t.Helper()

	return config.Load(filepath.Dir(files.Repo), filepath.Dir(files.Home))
}

func TestARepositoryFileLayersOverTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"jira": {"project": "OSS"}}`)

	// Act
	cfg, err := loadedFrom(t, files)

	// Assert
	if err != nil || cfg.Jira.Project != ossProject || cfg.Jira.BaseURL != jiraURL {
		t.Errorf("jira = %+v, %v; want the repository's project over the home file's address", cfg.Jira, err)
	}

	if cfg.Messaging.Channel != "#dev" || cfg.Files != files || cfg.Path != files.Repo {
		t.Errorf("channel %q, files %+v, path %q; want the home file's channel, both files, saving to the repository's",
			cfg.Messaging.Channel, cfg.Files, cfg.Path)
	}
}

func TestALayerReplacesAListWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"jira": {"views": [{"name": "OSS", "jql": "project = OSS"}]}}`)

	// Act
	cfg, err := loadedFrom(t, files)

	// Assert
	if err != nil || len(cfg.Jira.Views) != 1 || cfg.Jira.Views[0].Name != "OSS" {
		t.Errorf("views = %+v, %v; want the repository's one view alone", cfg.Jira.Views, err)
	}
}

func TestAnExplicitFalseInTheRepositoryFileOverridesHome(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"ui": {"ascii": false}}`)

	// Act
	cfg, err := loadedFrom(t, files)

	// Assert
	if err != nil || cfg.UI.ASCII {
		t.Errorf("ui.ascii = %v, %v; want the repository's false over the home file's true", cfg.UI.ASCII, err)
	}
}

func TestAnUnknownKeyNamesTheFileItIsIn(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"jira": {"projekt": "OSS"}}`)

	// Act
	_, err := loadedFrom(t, files)

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !strings.Contains(err.Error(), files.Repo) {
		t.Errorf("Load = %v, want the unknown key refused naming %s", err, files.Repo)
	}
}

func TestTheLayersAreValidatedTogether(t *testing.T) {
	t.Parallel()

	// Arrange
	// Each file alone is valid; together they set up Slack twice.
	files := layered(t, `{"messaging": {"client_id": "1234.5678"}}`)

	// Act
	_, err := loadedFrom(t, files)

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("Load = %v, want the webhook and the user token together refused", err)
	}
}

func TestASaveToTheRepositoryFileWritesOnlyWhatDiffersFromHome(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"jira": {"project": "OSS"}}`)

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("LoadLayersAt: %v", err)
	}

	cfg.Messaging.Channel = "#oss"

	// Act
	_, err = config.SaveLayers(files, cfg, over)
	// Assert
	if err != nil {
		t.Fatalf("SaveLayers: %v", err)
	}

	written := readJSON(t, files.Repo)
	want := map[string]any{"jira": map[string]any{"project": "OSS"}, "messaging": map[string]any{"channel": "#oss"}}

	if !reflect.DeepEqual(written, want) {
		t.Errorf("the repository file holds %v, want only %v: nothing inherited, no home secret", written, want)
	}
}

func TestAnEditToTheHomeFileRefusesASaveOverTheLayers(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"jira": {"project": "OSS"}}`)

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("LoadLayersAt: %v", err)
	}

	write(t, filepath.Dir(files.Home), `{"ui": {"ascii": false}}`)

	// Act
	_, err = config.SaveLayers(files, cfg, over)

	// Assert
	if !errors.Is(err, config.ErrChangedOnDisk) {
		t.Errorf("SaveLayers = %v, want ErrChangedOnDisk after the home file changed", err)
	}
}

func TestWithNoHomeFileTheRepositoryFileIsWrittenWhole(t *testing.T) {
	t.Parallel()

	// Arrange
	files := config.Files{Repo: write(t, t.TempDir(), `{"jira": {"project": "OSS"}}`)}

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("LoadLayersAt: %v", err)
	}

	// Act
	_, err = config.SaveLayers(files, cfg, over)

	// Assert
	if _, whole := readJSON(t, files.Repo)["timing"]; err != nil || !whole {
		t.Errorf("SaveLayers = %v; want the lone file written with every setting", err)
	}
}

// readJSON decodes the JSON object in the file at path.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var decoded map[string]any

	err = json.Unmarshal(contents, &decoded)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}

	return decoded
}

func TestARepositoryOptsIntoItsForgeIssuesOverTheHomeDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layered(t, `{"issues": {"forge": true}}`)

	// Act
	cfg, err := loadedFrom(t, files)

	// Assert
	if err != nil || !cfg.Issues.Forge || cfg.Jira.BaseURL != jiraURL {
		t.Errorf("issues.forge = %v with Jira at %q, %v; want the repository's opt-in beside the home file's Jira",
			cfg.Issues.Forge, cfg.Jira.BaseURL, err)
	}
}
