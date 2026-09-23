// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errUnexpectedPrompt is a command reading from the prompt when the test wired
// none.
var errUnexpectedPrompt = errors.New("unexpected prompt read")

// run executes the command tree in dir and returns everything it printed.
//
// It changes the working directory, because that is the input the command tree
// reads; t.Chdir restores it when the test ends. These tests therefore do not
// call t.Parallel().
//
// The home directory and git's global and system configuration are the
// developer's own, and none of them may decide a result: a ~/.workflow.json
// would stand in for a missing file, and a global commit.gpgsign would fail a
// commit. Each run gets an empty home and no git configuration but its own.
func run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	return runGuided(t, dir, unusedPrompt(t), args...)
}

// runGuided is run with a prompt that answers `config init`'s questions.
func runGuided(t *testing.T, dir string, prompt cli.Prompt, args ...string) (string, error) {
	t.Helper()

	printed, err := runStreams(t, dir, prompt, args...)

	return printed.stdout + printed.stderr, err
}

// streams is what a command printed, stream by stream.
type streams struct {
	stdout string
	stderr string
}

// runStreams is runGuided with the two streams kept apart, for a test that
// says which one a line belongs on: the artifact on stdout, what is said about
// it on stderr.
func runStreams(t *testing.T, dir string, prompt cli.Prompt, args ...string) (streams, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer

	err := cli.Execute(args, &stdout, &stderr, prompt)

	return streams{stdout: stdout.String(), stderr: stderr.String()}, err
}

// unusedPrompt fails the test if a command reads from it: only the guided
// `config init` should, and its tests drive it with scripted() instead.
func unusedPrompt(t *testing.T) cli.Prompt {
	t.Helper()

	fail := func(string) (string, error) {
		t.Error("a command read from the prompt when none was expected")

		return "", errUnexpectedPrompt
	}

	return cli.Prompt{Line: fail, Secret: fail}
}

// scripted answers the guided prompts in order: visible lines from lines,
// secrets from secrets. A prompt past the end of its list reads as blank.
func scripted(lines, secrets []string) cli.Prompt {
	return cli.Prompt{Line: answersFrom(&lines), Secret: answersFrom(&secrets)}
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

// writeFile writes a configuration into dir, failing the test if it cannot.
func writeFile(t *testing.T, dir, contents string) string {
	t.Helper()

	path := filepath.Join(dir, config.FileName)

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	return path
}

func TestConfigInitWritesATemplate(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	output, err := run(t, dir, "config", "init", "--template")
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
	// Arrange
	dir := t.TempDir()

	const existing = `{"jira": {"base_url": "https://keep.example.com"}}`

	path := writeFile(t, dir, existing)

	// Act
	output, err := run(t, dir, "config", "init")

	// Assert
	if err == nil {
		t.Fatalf("expected an error, got none (%s)", output)
	}

	wantExit(t, err, 4)

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
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, `{"jira": {"base_url": "https://old.example.com"}}`)

	// Act
	output, err := run(t, dir, "config", "init", "--template", "--force")
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
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, `{}`)

	err := os.Chmod(path, 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Act
	output, err := run(t, dir, "config", "init", "--template", "--force")

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
	// Arrange
	dir := t.TempDir()

	const secret = "xoxb-super-secret-9999"

	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "jira-secret-1111"},`+
		` "messaging": {"token": "`+secret+`", "channel": "#dev"}}`)

	// Act
	output, err := run(t, dir, "config", "show")
	if err != nil {
		t.Fatalf("config show: %v (%s)", err, output)
	}

	// Assert
	if strings.Contains(output, secret) || strings.Contains(output, "jira-secret") {
		t.Errorf("config show leaked a token:\n%s", output)
	}

	if !strings.Contains(output, "#dev") || !strings.Contains(output, "****9999") {
		t.Errorf("config show hid a non-secret value, or the masked token's tail:\n%s", output)
	}
}

func TestHelpExplainsBothTokens(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("--help: %v", err)
	}

	// Assert
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
	// Arrange
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/secretpath1234"

	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "`+webhook+`"}}`)

	// Act
	output, err := run(t, dir, "config", "show")
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

func TestConfigShowNamesHowToCreateAConfiguration(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "config", "show")
	// Assert
	if err != nil {
		t.Fatalf("config show = %v, want it to guide rather than fail:\n%s", err, output)
	}

	if !strings.Contains(output, "workflow config init") {
		t.Errorf("config show does not name the command that creates a configuration:\n%s", output)
	}
}

func TestEverySurfaceNamesBothSetupSteps(t *testing.T) {
	cases := map[string][]string{
		"config show": {"config", "show"},
		"doctor":      {"doctor"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()

			// Act
			output, _ := run(t, dir, args...)

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
	// Act
	output, err := run(t, t.TempDir(), "--version")
	// Assert
	if err != nil {
		t.Fatalf("--version returned %v, want nil", err)
	}

	if want := buildinfo.Current(); !strings.Contains(output, want) {
		t.Errorf("--version = %q, want it to contain the build %q", output, want)
	}
}

func TestDoctorReportsTheVersionFirst(t *testing.T) {
	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
	version, repository := strings.Index(output, "Version:"), strings.Index(output, "Repository:")
	if version < 0 || repository < 0 || version > repository {
		t.Errorf("doctor did not report the version before the repository:\n%s", output)
	}

	if got := fieldValue(output, "Version"); got != buildinfo.Current() {
		t.Errorf("doctor Version = %q, want %q", got, buildinfo.Current())
	}
}
