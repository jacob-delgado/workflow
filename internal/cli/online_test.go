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

// jiraFixture is the body Jira Data Center returns for /rest/api/2/myself.
const jiraFixture = `{"key":"JIRAUSER10100","name":"fred","displayName":"Fred F. User","active":true}`

// writeConfigFor writes a configuration pointing Jira at baseURL.
func writeConfigFor(t *testing.T, dir, baseURL string) {
	t.Helper()

	contents := `{"jira": {"base_url": "` + baseURL + `", "token": "jira-token-for-tests"},` +
		` "slack": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`

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

func TestDoctorOnlineSaysAWebhookCannotBeChecked(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	writeConfigFor(t, dir, server.URL)

	output, err := run(t, dir, "doctor", "--online")
	if err != nil {
		t.Fatalf("doctor --online failed on a webhook it merely cannot check: %v (%s)", err, output)
	}

	// Nothing is wrong with the configuration — there is simply nothing to ask,
	// because the only way to test a webhook is to post into someone's channel.
	if !strings.Contains(output, "cannot be checked") {
		t.Errorf("doctor does not explain why the webhook went unchecked:\n%s", output)
	}
}

func TestDoctorOnlineFailsWhenSlackIsNotConfigured(t *testing.T) {
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)

	contents := `{"jira": {"base_url": "` + server.URL + `", "token": "jira-token-for-tests"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	output, runErr := run(t, dir, "doctor", "--online")
	if runErr == nil {
		t.Fatalf("doctor --online accepted a missing Slack credential:\n%s", output)
	}

	if !strings.Contains(output, "no slack.token or slack.webhook_url") {
		t.Errorf("doctor does not name the missing Slack credential:\n%s", output)
	}
}

// forgeEnvironment is every variable the resolver consults, cleared so a test
// sees the same thing on a laptop and in CI — where GITHUB_TOKEN is often set.
func clearForgeEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "GLAB_TOKEN"} {
		t.Setenv(name, "")
	}
}

// pathWithOnlyGit points PATH at a directory holding nothing but git, so the
// forge CLI is absent while doctor can still read the repository. Emptying PATH
// outright would take git with it, and then there is no remote to resolve a
// token for in the first place.
func pathWithOnlyGit(t *testing.T) {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("these tests need git: %v", err)
	}

	dir := t.TempDir()

	err = os.Symlink(gitPath, filepath.Join(dir, "git"))
	if err != nil {
		t.Fatalf("linking git: %v", err)
	}

	t.Setenv("PATH", dir)
}

// repoWithConfig makes a repository with an origin and a configuration in it.
func repoWithConfig(t *testing.T, forgeToken string) string {
	t.Helper()

	dir := repoWithRemote(t, "git@github.com:owner/repo.git")

	contents := `{"jira": {"base_url": "https://jira.example.com", "token": "t"},` +
		` "slack": {"webhook_url": "https://hooks.slack.example/services/not-real"},` +
		` "forge": {"token": "` + forgeToken + `"}}`

	err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	return dir
}

func TestDoctorOnlineNamesWhereTheForgeTokenCameFrom(t *testing.T) {
	clearForgeEnvironment(t)
	t.Setenv("GITHUB_TOKEN", "forge-token-for-tests")

	dir := repoWithConfig(t, "")

	output, _ := run(t, dir, "doctor", "--online")

	if !strings.Contains(output, "token from the environment") {
		t.Errorf("doctor does not say where the forge token came from:\n%s", output)
	}

	// The source is the useful part. The token itself never is.
	if strings.Contains(output, "forge-token-for-tests") {
		t.Errorf("doctor printed the forge token:\n%s", output)
	}
}

func TestDoctorOnlinePrefersTheEnvironmentOverTheConfiguration(t *testing.T) {
	clearForgeEnvironment(t)
	t.Setenv("GH_TOKEN", "from-the-environment")

	dir := repoWithConfig(t, "from-the-file")

	output, _ := run(t, dir, "doctor", "--online")

	if !strings.Contains(output, "token from the environment") {
		t.Errorf("the configuration won over the environment:\n%s", output)
	}
}

func TestDoctorOnlineFallsBackToTheConfiguredForgeToken(t *testing.T) {
	clearForgeEnvironment(t)
	// Without gh on PATH the configuration is the last source standing.
	pathWithOnlyGit(t)

	dir := repoWithConfig(t, "from-the-file")

	output, _ := run(t, dir, "doctor", "--online")

	if !strings.Contains(output, "token from forge.token") {
		t.Errorf("doctor did not fall back to the configured token:\n%s", output)
	}
}

func TestDoctorOnlineReportsNoForgeTokenAtAll(t *testing.T) {
	clearForgeEnvironment(t)
	pathWithOnlyGit(t)

	dir := repoWithConfig(t, "")

	output, err := run(t, dir, "doctor", "--online")
	if err == nil {
		t.Fatalf("doctor accepted a missing forge token:\n%s", output)
	}

	// Naming all three sources is the point: the reader should not have to go
	// looking for which one they were supposed to use.
	for _, want := range []string{"GITHUB_TOKEN", "gh auth login", "forge.token"} {
		if !strings.Contains(output, want) {
			t.Errorf("doctor does not mention %q:\n%s", want, output)
		}
	}
}

func TestDoctorOnlineSaysThereIsNoForgeWithoutARemote(t *testing.T) {
	clearForgeEnvironment(t)

	dir := t.TempDir()
	gitInit(t, dir)
	writeConfigFor(t, dir, "https://jira.example.com")

	output, _ := run(t, dir, "doctor", "--online")

	if !strings.Contains(output, "no repository remote") {
		t.Errorf("doctor does not explain the absent forge:\n%s", output)
	}
}
