// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
)

// runAtHome is clitest.Run with dir as the home directory as well, so the
// file there is the home file, the one file that may set a program to run or a
// variable to read.
func runAtHome(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: dir, Home: dir}, clitest.UnusedPrompt(t), args...)

	return printed.Stdout + printed.Stderr, err
}

// scripted answers the guided prompts in order: visible lines from lines,
// secrets from secrets. A prompt past the end of its list reads as blank.
func scripted(lines, secrets []string) cmdkit.Prompt {
	return cmdkit.Prompt{Line: answersFrom(&lines), Secret: answersFrom(&secrets)}
}

// answering is a prompt that gives every question the same answer and keeps
// each question it was asked, for a test of what a command asks.
func answering(answer string, asked *[]string) cmdkit.Prompt {
	reply := func(question string) (string, error) {
		*asked = append(*asked, question)

		return answer, nil
	}

	return cmdkit.Prompt{Line: reply, Secret: reply}
}

// answersFrom hands back each answer in turn, then blanks.
func answersFrom(answers *[]string) func(string) (string, error) {
	return func(string) (string, error) {
		if len(*answers) == 0 {
			return "", nil
		}

		next := (*answers)[0]
		*answers = (*answers)[1:]

		return next, nil
	}
}

func TestConfigInitWritesATemplate(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	output, err := clitest.Run(t, dir, "config", "init", "--template")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
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
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	const existing = `{"jira": {"base_url": "https://keep.example.com"}}`

	path := clitest.WriteFile(t, dir, existing)

	// Act
	output, err := clitest.Run(t, dir, "config", "init")

	// Assert
	if err == nil {
		t.Fatalf("expected an error, got none (%s)", output)
	}

	wantExit(t, err, 4)

	if !strings.HasSuffix(err.Error(), "; pass --force to overwrite") {
		t.Errorf("config init = %v, want the hint after a semicolon", err)
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
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://old.example.com"}}`)

	// Act
	output, err := clitest.Run(t, dir, "config", "init", "--template", "--force")
	if err != nil {
		t.Fatalf("config init --template --force: %v (%s)", err, output)
	}

	// Assert
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if strings.Contains(string(contents), "old.example.com") ||
		!strings.Contains(string(contents), config.Template().Jira.BaseURL) {
		t.Errorf("file was not overwritten with the template:\n%s", contents)
	}
}

func TestConfigInitForceLeavesTheModeItReports(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{}`)

	err := os.Chmod(path, 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Act
	output, err := clitest.Run(t, dir, "config", "init", "--template", "--force")

	// Assert
	if err != nil || !strings.Contains(output, "mode 0600") {
		t.Fatalf("config init --template --force = %v, want it to report the mode:\n%s", err, output)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if info.Mode().Perm() != config.FileMode {
		t.Errorf("mode = %#o, though config init reported %#o", info.Mode().Perm(), config.FileMode)
	}
}

func TestConfigShowMasksTokens(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	const secret = "slack-refresh-secret-9999"

	clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "jira-secret-1111"},`+
		` "messaging": {"client_id": "1234.5678", "client_secret": "slack-client-secret-1234",`+
		` "refresh_token": "`+secret+`", "channel": "#dev"}}`)

	// Act
	output, err := clitest.Run(t, dir, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v (%s)", err, output)
	}

	// Assert
	if strings.Contains(output, secret) || strings.Contains(output, "jira-secret") ||
		strings.Contains(output, "slack-client-secret") {
		t.Errorf("config show leaked a token:\n%s", output)
	}

	if !strings.Contains(output, "#dev") || !strings.Contains(output, "****9999") {
		t.Errorf("config show hid a non-secret value, or the masked token's tail:\n%s", output)
	}
}

func TestHelpExplainsBothTokens(t *testing.T) {
	t.Parallel()

	// Act
	output, err := clitest.Run(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}

	// Assert
	// The help text is the only place a new user is told how to get credentials.
	wants := []string{
		"Personal Access Tokens",
		"xoxe.xoxp-",
		"chat:write",
		"users:read",
		"channels:read",
		"groups:read",
		"usergroups:read",
		"workflow slack login",
		config.FileName,
		"LAYERED",
		"up to the repository root",
		"Jira is optional",
		"answering the prompts",
		"--template",
	}

	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Errorf("--help does not mention %q", want)
		}
	}
}

func TestConfigShowMasksTheWebhookURL(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/secretpath1234"

	clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "`+webhook+`"}}`)

	// Act
	output, err := clitest.Run(t, dir, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v (%s)", err, output)
	}

	// Assert
	// A webhook URL is not a URL that contains a secret — it IS the secret.
	if strings.Contains(output, "hooks.slack.com") || strings.Contains(output, "secretpath") ||
		!strings.Contains(output, "****1234") {
		t.Errorf("config show did not mask the webhook URL to its tail:\n%s", output)
	}
}

func TestConfigShowReadsTheHomeConfigurationWhenTheDirectoryHasNone(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()
	path := clitest.WriteFile(t, home, `{"messaging": {"channel": "#from-home"}}`)

	// Act
	printed, err := clitest.RunStreamsAt(
		t, clitest.Place{Dir: t.TempDir(), Home: home}, clitest.UnusedPrompt(t), "config", "show",
	)
	if err != nil {
		t.Fatalf("config show: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.Contains(printed.Stdout, "#from-home") || !strings.Contains(printed.Stderr, path) {
		t.Errorf("config show did not fall back to the home directory's file %s:\nstdout:\n%s\nstderr:\n%s",
			path, printed.Stdout, printed.Stderr)
	}
}

func TestConfigShowNamesHowToCreateAConfiguration(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "config", "show")

	// Assert
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("config show = %v, want it to fail as having no configuration", err)
	}

	// The guidance is said about the missing file, so it is commentary, and a
	// script reading stdout gets nothing it could mistake for a configuration.
	if !strings.Contains(printed.Stderr, "workflow config init") || printed.Stdout != "" {
		t.Errorf("config show does not name, on stderr alone, the command that creates a configuration:"+
			"\nstdout:\n%s\nstderr:\n%s", printed.Stdout, printed.Stderr)
	}
}

func TestConfigShowExitsLikeDoctorWithoutAConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	_, doctorErr := clitest.Run(t, t.TempDir(), "doctor")

	// Act
	_, err := clitest.Run(t, t.TempDir(), "config", "show")

	// Assert
	if got, doctor := cli.ExitStatus(err), cli.ExitStatus(doctorErr); got != doctor || got != 3 {
		t.Errorf("without a configuration config show exits %d and doctor %d, want both 3", got, doctor)
	}
}

func TestConfigShowWritesOnlyJSONToStdout(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"}}`)

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "config", "show")
	if err != nil {
		t.Fatalf("config show: %v (%+v)", err, printed)
	}

	// Assert
	var shown map[string]any

	err = json.Unmarshal([]byte(printed.Stdout), &shown)
	if err != nil {
		t.Errorf("config show's stdout is not JSON alone, so `| jq .` fails: %v\n%s", err, printed.Stdout)
	}

	// Which file is in effect is still said, where a person reads it.
	if !strings.Contains(printed.Stderr, path) {
		t.Errorf("config show does not name the file in effect on stderr:\n%s", printed.Stderr)
	}
}

func TestConfigShowRefusesAMalformedConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, "{not json")

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "config", "show")

	// Assert
	// A file that exists but cannot be read is not a missing one: nothing tells
	// the user to create a file they already have.
	if !errors.Is(err, config.ErrInvalid) || strings.Contains(printed.Stdout+printed.Stderr, "config init") {
		t.Errorf("config show = %v, want the malformed file refused without creation advice:\n%+v", err, printed)
	}
}

func TestEverySurfaceNamesBothSetupSteps(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"config show": {"config", "show"},
		"doctor":      {"doctor"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()

			// Act
			output, _ := clitest.Run(t, dir, args...)

			// Assert
			for _, step := range []string{"workflow config init", "workflow doctor"} {
				if !strings.Contains(output, step) {
					t.Errorf("%s does not name %q:\n%s", name, step, output)
				}
			}
		})
	}
}

func TestTheVersionFlagPrintsTheBuild(t *testing.T) {
	t.Parallel()

	// Act
	output, err := clitest.Run(t, t.TempDir(), "--version")
	// Assert
	if err != nil {
		t.Fatalf("--version returned %v, want nil", err)
	}

	if want := buildinfo.Current(); !strings.Contains(output, want) {
		t.Errorf("--version = %q, want it to contain the build %q", output, want)
	}
}

func TestDoctorReportsTheVersionFirst(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := clitest.Run(t, t.TempDir(), "doctor")

	// Assert
	version, repository := strings.Index(output, "Version:"), strings.Index(output, "Repository:")
	if version < 0 || repository < 0 || version > repository {
		t.Errorf("doctor did not report the version before the repository:\n%s", output)
	}

	if got := fieldValue(output, "Version"); got != buildinfo.Current() {
		t.Errorf("doctor Version = %q, want %q", got, buildinfo.Current())
	}
}
