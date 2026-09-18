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

// guidedToken is the Jira token these guided-init tests type at the prompt.
const guidedToken = "jira-token-for-tests"

func TestGuidedInitWritesWhatChecksOut(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)
	prompt := scripted(
		[]string{jiraURL},
		[]string{guidedToken, "https://hooks.slack.example/x"},
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

	if cfg.Jira.BaseURL != jiraURL || cfg.Jira.Token != guidedToken || cfg.Slack.WebhookURL == "" {
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

func TestGuidedInitStoresTheTokenInTheKeychainWhenChosen(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)

	var stored atomic.Value

	prompt := scripted([]string{jiraURL, "y"}, []string{guidedToken})
	prompt.StoreSecret = func(secret string) (string, error) {
		stored.Store(secret)

		return "security find-generic-password -s workflow-jira -w", nil
	}

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

	if cfg.Jira.Token != "" || cfg.Jira.TokenCommand == "" || stored.Load() != guidedToken {
		t.Errorf("token not moved to the keychain: token=%q command=%q stored=%v",
			cfg.Jira.Token, cfg.Jira.TokenCommand, stored.Load())
	}
}

func TestGuidedInitKeepsTheTokenInTheFileWhenKeychainDeclined(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)

	prompt := scripted([]string{jiraURL, "n"}, []string{guidedToken})
	prompt.StoreSecret = func(string) (string, error) {
		t.Error("stored the token though the offer was declined")

		return "", errPromptBroke
	}

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

	if cfg.Jira.Token != guidedToken || cfg.Jira.TokenCommand != "" {
		t.Errorf("token not kept in the file: token=%q command=%q", cfg.Jira.Token, cfg.Jira.TokenCommand)
	}
}

func TestGuidedInitReportsAKeychainFailure(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)

	prompt := scripted([]string{jiraURL, "y"}, []string{guidedToken})
	prompt.StoreSecret = func(string) (string, error) { return "", errPromptBroke }

	// Act
	_, err := runGuided(t, dir, prompt, "config", "init")

	// Assert
	if !errors.Is(err, errPromptBroke) {
		t.Errorf("config init returned %v, want the keychain failure surfaced", err)
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
