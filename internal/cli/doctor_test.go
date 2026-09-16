// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestDoctorReportsAMissingFile(t *testing.T) {
	output, err := run(t, t.TempDir(), "doctor")
	if err == nil {
		t.Fatalf("expected an error, got none (%s)", output)
	}

	if !strings.Contains(output, "config init") {
		t.Errorf("output does not say how to create a config:\n%s", output)
	}
}

func TestDoctorNamesMissingFields(t *testing.T) {
	dir := t.TempDir()

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "t"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, err := run(t, dir, "doctor")
	if err == nil {
		t.Fatalf("expected an error for an incomplete config, got none (%s)", output)
	}

	if !strings.Contains(output, "slack.token or slack.webhook_url") {
		t.Errorf("doctor does not offer both Slack transports:\n%s", output)
	}

	// With no Slack credential at all, also demanding slack.channel would read
	// as "set three things" when either transport is the actual next step.
	if strings.Contains(output, "- slack.channel") {
		t.Errorf("doctor asked for a channel before a credential:\n%s", output)
	}
}

func TestDoctorAcceptsACompleteConfig(t *testing.T) {
	dir := t.TempDir()

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "t"},` +
		` "slack": {"token": "xoxb-t", "channel": "#dev"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v (%s)", err, output)
	}

	if !strings.Contains(output, "bearer token") {
		t.Errorf("doctor does not report the auth mode:\n%s", output)
	}
}

// gitInit makes dir a real repository, so doctor's repository section can be
// tested against git itself rather than against an imitation of it.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "--quiet", ".")
}

// git runs one git command in dir, failing the test if it does not succeed.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}
}

func TestDoctorReportsTheRepositoryItIsIn(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// The configuration is absent, so doctor exits non-zero. The repository
	// section must still be reported: someone runs doctor precisely when
	// something is wrong, and reporting only the first problem wastes the run.
	output, _ := run(t, dir, "doctor")

	for _, want := range []string{"Repository:", "Branch:", "Remote:"} {
		if !strings.Contains(output, want) {
			t.Errorf("doctor does not report %q:\n%s", want, output)
		}
	}

	// A freshly initialized repository has no commits, so its branch is unborn.
	// `git branch --show-current` still names it, which is why doctor can.
	if strings.Contains(output, "detached HEAD") {
		t.Errorf("doctor called an unborn branch detached:\n%s", output)
	}

	if !strings.Contains(output, "(not set)") {
		t.Errorf("doctor does not report the missing origin remote:\n%s", output)
	}
}

func TestDoctorReportsADirectoryOutsideAnyRepository(t *testing.T) {
	output, _ := run(t, t.TempDir(), "doctor")

	if !strings.Contains(output, "not in a git work tree") {
		t.Errorf("doctor does not say the directory is outside a repository:\n%s", output)
	}
}

func TestDoctorReportsTheExternalTooling(t *testing.T) {
	output, _ := run(t, t.TempDir(), "doctor")

	if !strings.Contains(output, "Tooling:") {
		t.Fatalf("doctor has no tooling section:\n%s", output)
	}

	// git is required and is present wherever these tests can run at all, so it
	// must be reported as found rather than merely mentioned.
	if !strings.Contains(output, "git        found") {
		t.Errorf("doctor does not report git as found:\n%s", output)
	}

	for _, program := range []string{"lefthook", "gh", "glab"} {
		if !strings.Contains(output, program) {
			t.Errorf("doctor does not mention the optional program %q:\n%s", program, output)
		}
	}
}

func TestDoctorReportsADetachedHead(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	// A commit is needed before HEAD can be detached from anything.
	git(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
		"commit", "--allow-empty", "--quiet", "--message", "seed")
	git(t, dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "HEAD")

	output, _ := run(t, dir, "doctor")

	if !strings.Contains(output, "detached HEAD") {
		t.Errorf("doctor does not report a detached HEAD:\n%s", output)
	}
}

func TestDoctorFailsWhenRequiredToolingIsMissing(t *testing.T) {
	// An empty PATH is the only portable way to make every program unfindable,
	// which is what proves the required/optional distinction actually bites.
	t.Setenv("PATH", "")

	output, err := run(t, t.TempDir(), "doctor")
	if err == nil {
		t.Fatalf("doctor succeeded with git missing:\n%s", output)
	}

	if !strings.Contains(output, "MISSING") {
		t.Errorf("doctor does not mark the missing required program:\n%s", output)
	}

	if !strings.Contains(err.Error(), "git") {
		t.Errorf("doctor error = %v, want it to name git", err)
	}

	// An optional program that is absent reads differently from a required one.
	if !strings.Contains(output, "not found") {
		t.Errorf("doctor does not distinguish an absent optional program:\n%s", output)
	}
}

func TestDoctorReportsAMalformedConfiguration(t *testing.T) {
	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("{not json"), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, runErr := run(t, dir, "doctor")
	if runErr == nil {
		t.Fatalf("doctor accepted a malformed configuration:\n%s", output)
	}

	// A file that exists but cannot be parsed is a different problem from no
	// file at all, and telling someone to create one they already have is worse
	// than saying nothing.
	if strings.Contains(output, "config init") {
		t.Errorf("doctor told the user to create a file that already exists:\n%s", output)
	}
}

func TestDoctorAcceptsAWebhookWithoutAChannel(t *testing.T) {
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/supersecretpayload"

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "t"},` +
		` "slack": {"webhook_url": "` + webhook + `"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, runErr := run(t, dir, "doctor")
	if runErr != nil {
		t.Fatalf("doctor rejected a webhook-only configuration: %v (%s)", runErr, output)
	}

	// The whole point of the leak test: doctor's output is what
	// .github/ISSUE_TEMPLATE/bug_report.yml invites people to paste.
	if strings.Contains(output, webhook) || strings.Contains(output, "supersecretpayload") {
		t.Errorf("doctor printed the webhook URL:\n%s", output)
	}

	if strings.Contains(output, "hooks.slack.com") {
		t.Errorf("doctor printed part of the webhook URL:\n%s", output)
	}

	if !strings.Contains(output, "incoming webhook") {
		t.Errorf("doctor does not name the Slack transport:\n%s", output)
	}
}

// jiraFixture is the body Jira Data Center returns for /rest/api/2/myself.
const jiraFixture = `{"key":"JIRAUSER10100","name":"fred","displayName":"Fred F. User","active":true}`

// writeConfigFor writes a configuration pointing Jira at baseURL.
func writeConfigFor(t *testing.T, dir, baseURL string) {
	t.Helper()

	contents := `{"jira": {"base_url": "` + baseURL + `", "token": "jira-token-for-tests"},` +
		` "slack": {"token": "xoxb-t", "channel": "#dev"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
}

// jiraServer answers one myself request with status and body, recording whether
// it was reached at all. The flag is atomic because `task test` runs -race and
// the handler runs on its own goroutine.
func jiraServer(t *testing.T, status int, body string, reached *atomic.Bool) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		reached.Store(true)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server
}

func TestDoctorDoesNotTouchTheNetworkWithoutTheFlag(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	writeConfigFor(t, dir, server.URL)

	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v (%s)", err, output)
	}

	if reached.Load() {
		t.Errorf("doctor called Jira without --online:\n%s", output)
	}

	if !strings.Contains(output, "--online") {
		t.Errorf("doctor does not say how to check the credentials:\n%s", output)
	}
}

func TestDoctorOnlineReportsTheJiraUser(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	writeConfigFor(t, dir, server.URL)

	output, err := run(t, dir, "doctor", "--online")
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	if !reached.Load() {
		t.Fatalf("doctor --online never called Jira:\n%s", output)
	}

	if !strings.Contains(output, "Fred F. User") || !strings.Contains(output, "fred") {
		t.Errorf("doctor does not name the authenticated user:\n%s", output)
	}

	if strings.Contains(output, "jira-token-for-tests") {
		t.Errorf("doctor printed the token:\n%s", output)
	}
}

func TestDoctorOnlineFailsOnARejectedCredential(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusUnauthorized, "<html>login</html>", &reached)
	writeConfigFor(t, dir, server.URL)

	output, err := run(t, dir, "doctor", "--online")
	if err == nil {
		t.Fatalf("doctor --online accepted a rejected credential:\n%s", output)
	}

	if !strings.Contains(output, "not accepted") {
		t.Errorf("doctor does not explain the rejection:\n%s", output)
	}

	if strings.Contains(output, "jira-token-for-tests") {
		t.Errorf("doctor printed the token:\n%s", output)
	}
}

func TestDoctorOnlineFallsBackToTheLoginName(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	// An instance can be configured to withhold display names.
	server := jiraServer(t, http.StatusOK, `{"name":"fred","active":true}`, &reached)
	writeConfigFor(t, dir, server.URL)

	output, err := run(t, dir, "doctor", "--online")
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	if !strings.Contains(output, "fred") {
		t.Errorf("doctor does not fall back to the login name:\n%s", output)
	}
}
