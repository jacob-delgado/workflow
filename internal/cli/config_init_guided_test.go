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

	if cfg.Jira.BaseURL != jiraURL || cfg.Jira.Token != guidedToken || cfg.Messaging.WebhookURL == "" {
		t.Errorf("wrote jira %+v, messaging %+v, want the checked values kept", cfg.Jira, cfg.Messaging)
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
	printed, err := runStreams(t, dir, prompt, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%+v)", err, printed)
	}

	// Assert
	// A warning is said about the file written, not part of anything a script
	// would capture, so it is on stderr alone.
	if !strings.Contains(printed.stderr, "not ignored by git") || strings.Contains(printed.stdout, "not ignored") {
		t.Errorf("expected a git-ignore warning on stderr alone:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
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
	where := place{dir: t.TempDir(), home: t.TempDir()}

	// Act
	printed, err := runStreamsAt(t, where, unusedPrompt(t), "config", "init", "--template", "--global")
	if err != nil {
		t.Fatalf("config init --template --global: %v (%+v)", err, printed)
	}

	// Assert
	_, err = os.Stat(filepath.Join(where.home, config.FileName))
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

func TestConfigInitDryRunWritesNoTemplate(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "config", "init", "--template", "--dry-run")
	if err != nil {
		t.Fatalf("config init --template --dry-run: %v (%+v)", err, printed)
	}

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, config.FileName))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a dry run left a file behind: %v", statErr)
	}

	shown, decodeErr := config.Parse(strings.NewReader(printed.stdout))
	if decodeErr != nil || shown.Jira.BaseURL != config.Template().Jira.BaseURL {
		t.Errorf("a dry run did not print the template it would write: %v\n%s", decodeErr, printed.stdout)
	}

	if !strings.Contains(printed.stderr, "dry run: would write") {
		t.Errorf("a dry run did not say what it would write:\n%s", printed.stderr)
	}
}

func TestGuidedInitDryRunWritesAndStoresNothing(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	jiraURL := workingJira(t)

	var stored atomic.Bool

	prompt := scripted([]string{jiraURL, "y"}, []string{guidedToken, "https://hooks.slack.example/x"})
	prompt.StoreSecret = func(string) (string, error) {
		stored.Store(true)

		return "security find-generic-password -s workflow-jira -w", nil
	}

	// Act
	printed, err := runStreams(t, dir, prompt, "config", "init", "--dry-run")
	if err != nil {
		t.Fatalf("config init --dry-run: %v (%+v)", err, printed)
	}

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, config.FileName))
	if !errors.Is(statErr, os.ErrNotExist) || stored.Load() {
		t.Errorf("a dry run wrote the file (%v) or stored the token in the keychain (%v)", statErr, stored.Load())
	}

	// What it would have written is shown, with the credential masked.
	shown, decodeErr := config.Parse(strings.NewReader(printed.stdout))
	if decodeErr != nil || shown.Jira.BaseURL != jiraURL || strings.Contains(printed.stdout, guidedToken) {
		t.Errorf("a dry run did not print the masked file it would write: %v\n%s", decodeErr, printed.stdout)
	}
}

func TestConfigInitDryRunRefusesAnExistingFile(t *testing.T) {
	// A dry run previews what would happen, and what would happen is a refusal.
	cases := [][]string{
		strings.Fields("config init --template --dry-run"),
		strings.Fields("config init --dry-run"),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			existing := `{"jira":{"base_url":"https://jira.example.com"}}`
			path := writeFile(t, dir, existing)

			// Act
			// No prompt: the guided flow refuses before it asks anything.
			printed, err := runStreams(t, dir, unusedPrompt(t), args...)

			// Assert
			wantExit(t, err, 4)

			if err == nil || !strings.Contains(err.Error(), "already exists") || printed.stdout != "" {
				t.Errorf("%s over a file = %v, want it refused with no preview:\n%s", name, err, printed.stdout)
			}

			kept, readErr := os.ReadFile(path)
			if readErr != nil || string(kept) != existing {
				t.Errorf("%s changed the file: %q (%v)", name, kept, readErr)
			}
		})
	}
}
