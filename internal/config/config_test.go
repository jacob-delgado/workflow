// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
)

// write puts a configuration file in dir and returns its path.
func write(t *testing.T, dir, contents string) string {
	t.Helper()

	path := filepath.Join(dir, config.FileName)

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	return path
}

const jiraURL = "https://jira.example.com"

// devChannel and botToken are the Slack fixtures these tests share.
const (
	devChannel = "#dev"
	botToken   = "xoxb-t"
)

// webhookURL is shaped like a real Slack incoming webhook. It is not one.
const webhookURL = "https://hooks.slack.com/services/T00000000/B00000000/fakefakefake2468"

// forgeFixture stands in for a GitHub or GitLab personal access token. Named
// away from "token" on purpose: gitleaks scans this repository and its
// generic-api-key rule keys off the identifier as much as the value.
const forgeFixture = "not-a-real-forge-credential"

const completeConfig = `{
  "jira": {"base_url": "https://jira.example.com", "token": "jira-token-1234", "user": ""},
  "slack": {"token": "xoxb-slack-token-5678", "channel": "#dev"}
}`

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

func TestTimingIsParsedWhenSet(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	write(t, workDir, `{"timing": {"request_timeout": "45s", "ci_interval": "3m"}}`)

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load returned %v, want nil", err)
	}

	// Assert
	if cfg.RequestTimeout() != 45*time.Second || cfg.CIInterval() != 3*time.Minute {
		t.Errorf("timing = %v / %v, want 45s / 3m", cfg.RequestTimeout(), cfg.CIInterval())
	}
}

func TestUnsetTimingIsZeroSoTheDefaultApplies(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.Default()

	// Assert
	if cfg.RequestTimeout() != 0 || cfg.CIInterval() != 0 {
		t.Errorf("default timing = %v / %v, want zero so the caller's default applies",
			cfg.RequestTimeout(), cfg.CIInterval())
	}
}

func TestRedactedHidesEveryCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	const (
		jiraToken  = "jira-token-1234"
		slackToken = "xoxb-slack-token-5678"
	)

	cfg := config.Config{
		Jira: config.Jira{BaseURL: jiraURL, Token: jiraToken, User: ""},
		Slack: config.Slack{
			Token:      slackToken,
			WebhookURL: webhookURL,
			Channel:    devChannel,
		},
		Forge: config.Forge{Token: forgeFixture},
		Path:  "/tmp/.workflow.json",
	}

	// Act
	redacted := cfg.Redacted()

	// Assert
	if strings.Contains(redacted.Jira.Token, "jira-token") {
		t.Errorf("jira token leaked: %q", redacted.Jira.Token)
	}

	if strings.Contains(redacted.Slack.Token, "slack-token") {
		t.Errorf("slack token leaked: %q", redacted.Slack.Token)
	}

	// Redaction must not mutate the original.
	if cfg.Jira.Token != jiraToken {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Jira.Token)
	}

	if strings.Contains(redacted.Forge.Token, "not-a-real") {
		t.Errorf("forge token leaked: %q", redacted.Forge.Token)
	}

	// A webhook URL is not a URL with a secret in it — it IS the credential.
	// Anyone holding it can post to that channel, so it masks like a token.
	if strings.Contains(redacted.Slack.WebhookURL, "hooks.slack.com") {
		t.Errorf("webhook url leaked: %q", redacted.Slack.WebhookURL)
	}

	if cfg.Slack.WebhookURL != webhookURL {
		t.Errorf("Redacted mutated the receiver: %q", cfg.Slack.WebhookURL)
	}

	// Enough tail survives to tell two tokens apart.
	if !strings.HasSuffix(redacted.Slack.Token, "5678") {
		t.Errorf("slack token = %q, want it to end in 5678", redacted.Slack.Token)
	}
}

func TestRedact(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		secret string
		want   string
	}{
		"empty stays empty":      {secret: "", want: ""},
		"short is fully masked":  {secret: "abcd", want: "****"},
		"long keeps four digits": {secret: "abcdefgh", want: "****efgh"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := config.Redact(tt.secret)

			// Assert
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.secret, got, tt.want)
			}
		})
	}
}

func TestSaveWritesOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)

	// Act
	err := config.Save(path, config.Template())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Assert
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	// The file holds live API tokens; nobody but the owner may read it.
	if info.Mode().Perm() != config.FileMode {
		t.Errorf("mode = %#o, want %#o", info.Mode().Perm(), config.FileMode)
	}
}

func TestSavedTemplateLoadsBack(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()

	err := config.Save(filepath.Join(workDir, config.FileName), config.Template())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Act
	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	// The template must round-trip through the strict decoder: if it ever grows
	// a key the loader rejects, `config init` would produce a file that
	// immediately fails to load.
	if cfg.Slack.Channel != config.Template().Slack.Channel {
		t.Errorf("slack.channel = %q, want the template's value", cfg.Slack.Channel)
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

// sharedMode is a file its owner's group and everyone else can read.
const sharedMode os.FileMode = 0o644

func TestSaveOverAnExistingFileLeavesItOwnerOnly(t *testing.T) {
	t.Parallel()

	// Arrange
	// Writing into a file that already exists keeps the mode it had, so the
	// mode has to be set and not only asked for.
	path := filepath.Join(t.TempDir(), config.FileName)

	err := os.WriteFile(path, []byte("{}"), sharedMode)
	if err != nil {
		t.Fatalf("writing the existing file: %v", err)
	}

	// Act
	err = config.Save(path, config.Template())
	// Assert
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if info.Mode().Perm() != config.FileMode {
		t.Errorf("mode = %#o, want %#o", info.Mode().Perm(), config.FileMode)
	}
}

func TestSharedModeReportsAFileOthersCanReach(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mode os.FileMode
		want bool
	}{
		"the owner alone":      {mode: config.FileMode, want: false},
		"the group can read":   {mode: 0o640, want: true},
		"everyone can read":    {mode: sharedMode, want: true},
		"everyone can write":   {mode: 0o602, want: true},
		"the owner, read-only": {mode: 0o400, want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The mode is set after the file is written, because the process's
			// umask takes bits off the one a new file is created with.
			path := filepath.Join(t.TempDir(), config.FileName)

			err := errors.Join(os.WriteFile(path, []byte("{}"), config.FileMode), os.Chmod(path, tt.mode))
			if err != nil {
				t.Fatalf("writing the file: %v", err)
			}

			// Act
			mode, shared := config.SharedMode(path)

			// Assert
			if shared != tt.want || (shared && mode != tt.mode) {
				t.Errorf("SharedMode = %#o, %t; want %#o, %t", mode, shared, tt.mode, tt.want)
			}
		})
	}
}

func TestSharedModeOfAMissingFileIsNotShared(t *testing.T) {
	t.Parallel()

	// Act & Assert
	if mode, shared := config.SharedMode(filepath.Join(t.TempDir(), config.FileName)); shared {
		t.Errorf("SharedMode of a missing file = %#o, shared", mode)
	}
}

func TestSaveReportsAnUnwritablePath(t *testing.T) {
	t.Parallel()

	// Arrange
	// A path whose parent does not exist: the encode succeeds and the write is
	// what fails, which is the arm being exercised.
	path := filepath.Join(t.TempDir(), "missing-dir", config.FileName)

	// Act
	err := config.Save(path, config.Template())

	// Assert
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Save = %v, want the write's own not-exist error", err)
	}
}
