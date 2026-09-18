// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errPromptBroke is a prompt that cannot read, such as a closed input.
var errPromptBroke = errors.New("prompt failed")

func TestGuidedInitWritesWhatChecksOut(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)
	prompt := scripted(
		[]string{jiraURL},
		[]string{"jira-token-for-tests", "https://hooks.slack.example/x"},
	)

	// Act
	output, err := runGuided(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	cfg, err := config.Load(dir, t.TempDir())
	if err != nil {
		t.Fatalf("loading what was written: %v", err)
	}

	if cfg.Jira.BaseURL != jiraURL || cfg.Jira.Token != "jira-token-for-tests" || cfg.Slack.WebhookURL == "" {
		t.Errorf("wrote jira %+v, slack %+v, want the checked values kept", cfg.Jira, cfg.Slack)
	}
}

func TestGuidedInitSkipsAServiceThatFailsItsCheckWhenDeclined(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	badJira := jiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool)).URL
	prompt := scripted([]string{badJira, "n"}, []string{"bad-token"})

	// Act
	output, err := runGuided(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	cfg, err := config.Load(dir, t.TempDir())
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	if cfg.Jira.BaseURL != "" {
		t.Errorf("jira was saved though its check failed and was declined: %+v", cfg.Jira)
	}
}

func TestGuidedInitKeepsAFailedServiceWhenInsisted(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	badJira := jiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool)).URL
	prompt := scripted([]string{badJira, "y"}, []string{"insisted-token"})

	// Act
	output, err := runGuided(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	cfg, err := config.Load(dir, t.TempDir())
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	if cfg.Jira.BaseURL != badJira {
		t.Errorf("jira was not saved though the user insisted: %+v", cfg.Jira)
	}
}

func TestGuidedInitWarnsWhenTheFileIsNotGitIgnored(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)

	prompt := scripted([]string{""}, []string{""})

	// Act
	output, err := runGuided(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	if !strings.Contains(output, "not ignored by git") {
		t.Errorf("expected a git-ignore warning:\n%s", output)
	}
}

func TestGuidedInitReportsAPromptThatCannotRead(t *testing.T) {
	// Arrange
	broken := cli.Prompt{
		Line:   func(string) (string, error) { return "", errPromptBroke },
		Secret: func(string) (string, error) { return "", errPromptBroke },
	}

	// Act
	_, err := runGuided(t, t.TempDir(), broken, "config", "init")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Errorf("config init returned %v, want the prompt failure surfaced", err)
	}
}

func TestGuidedInitReportsAFailureReadingTheToken(t *testing.T) {
	// Arrange
	// The URL reads, but reading the secret token fails.
	broken := cli.Prompt{
		Line:   func(string) (string, error) { return "https://jira.example.com", nil },
		Secret: func(string) (string, error) { return "", errPromptBroke },
	}

	// Act
	_, err := runGuided(t, t.TempDir(), broken, "config", "init")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Errorf("config init returned %v, want the token-read failure surfaced", err)
	}
}

func TestConfigInitTemplateWritesGloballyToHome(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	output, err := run(t, dir, "config", "init", "--template", "--global")
	if err != nil {
		t.Fatalf("config init --template --global: %v (%s)", err, output)
	}

	// Assert
	home := os.Getenv("HOME")

	_, err = os.Stat(filepath.Join(home, config.FileName))
	if err != nil {
		t.Errorf("--global did not write to the home directory: %v", err)
	}
}

func TestGuidedInitStaysQuietWhenTheFileIsGitIgnored(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	gitInit(t, dir)

	err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(config.FileName+"\n"), 0o644)
	if err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}

	prompt := scripted([]string{""}, []string{""})

	// Act
	output, err := runGuided(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	if strings.Contains(output, "not ignored by git") {
		t.Errorf("warned about a file that is ignored:\n%s", output)
	}
}
