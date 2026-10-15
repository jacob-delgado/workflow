// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
)

// homeConfig is the configuration in the home directory the layering tests
// start a repository's file over.
const homeConfig = `{"jira": {"base_url": "https://jira.example.com", "token": "jira-token-home-1234"},` +
	` "messaging": {"webhook_url": "https://hooks.slack.example/home"}}`

// homeAndRepository is a home directory holding homeConfig and a repository
// beside it, with nothing in the repository yet.
func homeAndRepository(t *testing.T) clitest.Place {
	t.Helper()

	home := t.TempDir()
	clitest.WriteFile(t, home, homeConfig)

	repo := t.TempDir()

	err := os.Mkdir(filepath.Join(repo, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repository marker: %v", err)
	}

	return clitest.Place{Dir: repo, Home: home}
}

func TestConfigInitTemplateOverAHomeFileWritesAnEmptyLayer(t *testing.T) {
	t.Parallel()

	// Arrange
	where := homeAndRepository(t)

	// Act
	printed, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "config", "init", "--template")

	// Assert
	written, readErr := os.ReadFile(filepath.Join(where.Dir, config.FileName))
	if err != nil || readErr != nil || strings.TrimSpace(string(written)) != "{}" {
		t.Fatalf("config init = %v; wrote %q (%v); want an empty layer", err, written, readErr)
	}

	if !strings.Contains(printed.Stderr, filepath.Join(where.Home, config.FileName)) {
		t.Errorf("config init said:\n%s\nwant it to name the home file the layer lies over", printed.Stderr)
	}
}

func TestGuidedInitOverAHomeFileKeepsWhatWasLeftBlank(t *testing.T) {
	t.Parallel()

	// Arrange
	where := homeAndRepository(t)

	// Act
	_, err := clitest.RunStreamsAt(t, where, scripted([]string{""}, []string{""}), "config", "init")
	// Assert
	if err != nil {
		t.Fatalf("config init: %v", err)
	}

	cfg, err := config.Load(where.Dir, where.Home)
	if err != nil || cfg.Jira.Token != "jira-token-home-1234" || cfg.Messaging.WebhookURL == "" {
		t.Errorf("loaded %+v, %v; want the home file's Jira and webhook kept", cfg, err)
	}
}

func TestConfigShowNamesEveryFileInEffect(t *testing.T) {
	t.Parallel()

	// Arrange
	where := homeAndRepository(t)
	clitest.WriteFile(t, where.Dir, `{"jira": {"project": "OSS"}}`)

	// Act
	printed, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "config", "show")

	// Assert
	home, repo := filepath.Join(where.Home, config.FileName), filepath.Join(where.Dir, config.FileName)
	if err != nil || !strings.Contains(printed.Stderr, home) || !strings.Contains(printed.Stderr, repo) {
		t.Errorf("config show = %v, said:\n%s\nwant it to name %s and %s", err, printed.Stderr, repo, home)
	}
}

func TestDoctorNamesEveryFileInEffect(t *testing.T) {
	t.Parallel()

	// Arrange
	where := homeAndRepository(t)
	clitest.WriteFile(t, where.Dir, `{"jira": {"project": "OSS"}}`)

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor", "--json")

	// Assert
	var report struct {
		Configuration struct {
			Path  string   `json:"path"`
			Files []string `json:"files"`
		} `json:"configuration"`
	}

	err := json.Unmarshal([]byte(printed.Stdout), &report)

	home, repo := filepath.Join(where.Home, config.FileName), filepath.Join(where.Dir, config.FileName)
	got := report.Configuration

	if err != nil || got.Path != repo || !slices.Equal(got.Files, []string{home, repo}) {
		t.Errorf("configuration = %+v, %v; want saving to %s, read from %s then %s", got, err, repo, home, repo)
	}
}

func TestConfigInitForceOverAHomeFileReplacesTheRepositoryLayer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, command string
		prompt        func(t *testing.T) cmdkit.Prompt
	}{
		{
			name:    "template",
			command: "config init --template --force",
			prompt:  clitest.UnusedPrompt,
		},
		{
			name:    "guided",
			command: "config init --force",
			prompt:  func(*testing.T) cmdkit.Prompt { return scripted([]string{""}, []string{""}) },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			where := homeAndRepository(t)
			clitest.WriteFile(t, where.Dir, `{"jira": {"project": "OSS"}}`)

			// Act
			_, err := clitest.RunStreamsAt(t, where, testCase.prompt(t), strings.Fields(testCase.command)...)

			// Assert
			cfg, loadErr := config.Load(where.Dir, where.Home)
			if err != nil || loadErr != nil || cfg.Jira.Project != "" || cfg.Jira.Token != "jira-token-home-1234" {
				t.Errorf("config init = %v; loaded project %q, %v; want the layer replaced over the home file",
					err, cfg.Jira.Project, loadErr)
			}
		})
	}
}

func TestConfigInitTemplateOverAHomeFileWarnsWhenTheLayerIsNotGitIgnored(t *testing.T) {
	t.Parallel()

	// Arrange
	where := homeAndRepository(t)
	gitInit(t, where.Dir)

	// Act
	printed, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "config", "init", "--template")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "not ignored by git") {
		t.Errorf("config init --template = %v, said:\n%s\nwant a git-ignore warning for the layer", err, printed.Stderr)
	}
}

func TestConfigInitOverAHomeFileThatDoesNotParseSaysSoAndWritesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"the template":     strings.Fields("config init --template"),
		"the guided setup": strings.Fields("config init"),
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The repository's file would lie over the home file, which cannot
			// be read for what it would lie over.
			where := homeAndRepository(t)
			clitest.WriteFile(t, where.Home, `{`)

			// Act
			_, err := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), args...)

			// Assert
			home := filepath.Join(where.Home, config.FileName)
			if err == nil || !strings.Contains(err.Error(), home) {
				t.Errorf("config init over an unreadable home file = %v, want it to name %s", err, home)
			}

			noConfigWritten(t, where.Dir)
		})
	}
}

func TestConfigInitOverAHomeFileIntoADirectoryItCannotWriteSaysSo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args   []string
		prompt func(t *testing.T) cmdkit.Prompt
	}{
		"the template": {args: strings.Fields("config init --template"), prompt: clitest.UnusedPrompt},
		"the guided setup, every prompt skipped": {
			args:   strings.Fields("config init"),
			prompt: func(*testing.T) cmdkit.Prompt { return scripted([]string{""}, []string{""}) },
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			if os.Geteuid() == 0 {
				t.Skip("root writes into a directory whatever its mode, so the save would succeed")
			}

			where := homeAndRepository(t)

			err := os.Chmod(where.Dir, 0o500)
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() { _ = os.Chmod(where.Dir, 0o700) })

			// Act
			_, err = clitest.RunStreamsAt(t, where, tt.prompt(t), tt.args...)

			// Assert
			if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), config.FileName) {
				t.Errorf("config init into a sealed directory = %v, want the file it could not write named", err)
			}

			noConfigWritten(t, where.Dir)
		})
	}
}
