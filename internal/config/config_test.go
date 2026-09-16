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

const completeConfig = `{
  "jira": {"base_url": "https://jira.example.com", "token": "jira-token-1234", "user": ""},
  "slack": {"token": "xoxb-slack-token-5678", "channel": "#dev"}
}`

func TestLoadPrefersWorkingDirectoryOverHome(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	homeDir := t.TempDir()

	wantPath := write(t, workDir, `{"jira": {"base_url": "https://work.example.com"}}`)
	write(t, homeDir, completeConfig)

	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want %s", cfg.Path, wantPath)
	}

	if cfg.Jira.BaseURL != "https://work.example.com" {
		t.Errorf("jira.base_url = %q, want the working directory's value", cfg.Jira.BaseURL)
	}
}

// The working directory's file REPLACES the home one. If the two ever merged,
// the home file's Slack token would leak into this result.
func TestLoadDoesNotMergeHomeIntoWorkingDirectory(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	homeDir := t.TempDir()

	write(t, workDir, `{"jira": {"base_url": "https://work.example.com"}}`)
	write(t, homeDir, completeConfig)

	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Slack.Token != "" {
		t.Errorf("slack.token = %q, want empty: the home file must not merge in", cfg.Slack.Token)
	}
}

func TestLoadFallsBackToHome(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	homeDir := t.TempDir()
	wantPath := write(t, homeDir, completeConfig)

	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want %s", cfg.Path, wantPath)
	}

	if cfg.Slack.Channel != "#dev" {
		t.Errorf("slack.channel = %q, want #dev", cfg.Slack.Channel)
	}
}

func TestLoadReportsNotFound(t *testing.T) {
	t.Parallel()

	_, err := config.Load(t.TempDir(), t.TempDir())
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
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workDir := t.TempDir()
			write(t, workDir, contents)

			_, err := config.Load(workDir, t.TempDir())
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestJiraAuthMode(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		jira config.Jira
		want config.AuthMode
	}{
		"token only means bearer": {
			jira: config.Jira{BaseURL: jiraURL, Token: "t", User: ""},
			want: config.AuthBearer,
		},
		"token plus user means basic": {
			jira: config.Jira{BaseURL: jiraURL, Token: "t", User: "jacob"},
			want: config.AuthBasic,
		},
		"no token means none": {
			jira: config.Jira{BaseURL: jiraURL, Token: "", User: "jacob"},
			want: config.AuthNone,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tt.jira.AuthMode()
			if got != tt.want {
				t.Errorf("AuthMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMissingNamesEveryEmptyRequiredField(t *testing.T) {
	t.Parallel()

	var empty config.Config

	got := empty.Missing()
	want := []string{"jira.base_url", "jira.token", "slack.token", "slack.channel"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Missing() = %v, want %v", got, want)
	}
}

func TestMissingIsEmptyForCompleteConfig(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	write(t, workDir, completeConfig)

	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := cfg.Missing()
	if len(got) != 0 {
		t.Errorf("Missing() = %v, want none", got)
	}
}

func TestRedactedHidesBothTokens(t *testing.T) {
	t.Parallel()

	const (
		jiraToken  = "jira-token-1234"
		slackToken = "xoxb-slack-token-5678"
	)

	cfg := config.Config{
		Jira:  config.Jira{BaseURL: jiraURL, Token: jiraToken, User: ""},
		Slack: config.Slack{Token: slackToken, Channel: "#dev"},
		Path:  "/tmp/.workflow.json",
	}

	redacted := cfg.Redacted()

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

			got := config.Redact(tt.secret)
			if got != tt.want {
				t.Errorf("Redact(%q) = %q, want %q", tt.secret, got, tt.want)
			}
		})
	}
}

func TestSaveWritesOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), config.FileName)

	err := config.Save(path, config.Template())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

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

	workDir := t.TempDir()
	path := filepath.Join(workDir, config.FileName)

	err := config.Save(path, config.Template())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg, err := config.Load(workDir, t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// The template must round-trip through the strict decoder: if it ever grows
	// a key the loader rejects, `config init` would produce a file that
	// immediately fails to load.
	if cfg.Slack.Channel != config.Template().Slack.Channel {
		t.Errorf("slack.channel = %q, want the template's value", cfg.Slack.Channel)
	}
}

func TestDiscoverIgnoresAnEmptyDirectory(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	wantPath := write(t, homeDir, completeConfig)

	// A machine with no home directory hands Load an empty string rather than a
	// path. That must skip the entry, not turn into a lookup of "/.workflow.json".
	got, err := config.Discover("", homeDir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if got != wantPath {
		t.Errorf("Discover = %s, want %s", got, wantPath)
	}
}

func TestLoadFileReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	_, err := config.LoadFile(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("expected an error for a missing file, got none")
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestSaveReportsAnUnwritablePath(t *testing.T) {
	t.Parallel()

	// A path whose parent does not exist: the encode succeeds and the write is
	// what fails, which is the arm being exercised.
	path := filepath.Join(t.TempDir(), "missing-dir", config.FileName)

	err := config.Save(path, config.Template())
	if err == nil {
		t.Fatal("expected an error writing into a missing directory, got none")
	}
}

func TestAuthModeString(t *testing.T) {
	t.Parallel()

	cases := map[config.AuthMode]string{
		config.AuthNone:   "none",
		config.AuthBearer: "bearer token",
		config.AuthBasic:  "basic auth",
		// A value outside the enum: the String method must stay total rather than
		// returning an empty string that reads as "no auth configured".
		config.AuthMode(99): "unknown",
	}

	for mode, want := range cases {
		got := mode.String()
		if got != want {
			t.Errorf("AuthMode(%d).String() = %q, want %q", mode, got, want)
		}
	}
}
