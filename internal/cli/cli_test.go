// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
)

// run executes the command tree in dir and returns everything it printed.
//
// It changes the working directory, because that is the input the command tree
// reads; t.Chdir restores it when the test ends. These tests therefore do not
// call t.Parallel().
func run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer

	err := cli.Execute(args, &stdout, &stderr)

	return stdout.String() + stderr.String(), err
}

func TestConfigInitWritesATemplate(t *testing.T) {
	dir := t.TempDir()

	output, err := run(t, dir, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	path := filepath.Join(dir, config.FileName)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}

	if info.Mode().Perm() != config.FileMode {
		t.Errorf("mode = %#o, want %#o", info.Mode().Perm(), config.FileMode)
	}

	if !strings.Contains(output, "workflow doctor") {
		t.Errorf("output does not point at the next step:\n%s", output)
	}
}

func TestConfigInitRefusesToOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	const existing = `{"jira": {"base_url": "https://keep.example.com"}}`

	err := os.WriteFile(path, []byte(existing), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, err := run(t, dir, "config", "init")
	if err == nil {
		t.Fatalf("expected an error, got none (%s)", output)
	}

	// The refusal is the point: that file holds credentials that cannot be
	// recovered once overwritten.
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if string(contents) != existing {
		t.Errorf("file was overwritten:\n%s", contents)
	}
}

func TestConfigInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.FileName)

	err := os.WriteFile(path, []byte(`{"jira": {"base_url": "https://old.example.com"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, err := run(t, dir, "config", "init", "--force")
	if err != nil {
		t.Fatalf("config init --force: %v (%s)", err, output)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if strings.Contains(string(contents), "old.example.com") {
		t.Errorf("file was not overwritten:\n%s", contents)
	}
}

func TestConfigShowMasksTokens(t *testing.T) {
	dir := t.TempDir()

	const secret = "xoxb-super-secret-9999"

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "jira-secret-1111"},` +
		` "slack": {"token": "` + secret + `", "channel": "#dev"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, err := run(t, dir, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v (%s)", err, output)
	}

	if strings.Contains(output, secret) || strings.Contains(output, "jira-secret") {
		t.Errorf("config show leaked a token:\n%s", output)
	}

	if !strings.Contains(output, "#dev") {
		t.Errorf("config show hid a non-secret value:\n%s", output)
	}
}

func TestHelpExplainsBothTokens(t *testing.T) {
	output, err := run(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}

	// The help text is the only place a new user is told how to get credentials.
	wants := []string{
		"Personal Access Tokens",
		"xoxb-",
		"chat:write",
		config.FileName,
		"REPLACES",
	}

	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("--help does not mention %q", want)
		}
	}
}

func TestConfigShowMasksTheWebhookURL(t *testing.T) {
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/secretpath1234"

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "t"},` +
		` "slack": {"webhook_url": "` + webhook + `"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, runErr := run(t, dir, "config", "show")
	if runErr != nil {
		t.Fatalf("config show: %v (%s)", runErr, output)
	}

	// A webhook URL is not a URL that contains a secret — it IS the secret.
	if strings.Contains(output, "hooks.slack.com") || strings.Contains(output, "secretpath") {
		t.Errorf("config show leaked the webhook URL:\n%s", output)
	}
}
