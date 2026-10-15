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

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestDoctorOnlineReportsTheJiraUser(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err != nil || !reached.Load() {
		t.Fatalf("doctor --online = %v, reached Jira %v:\n%s", err, reached.Load(), output)
	}

	if !strings.Contains(output, "authenticates as Fred F. User (fred)") {
		t.Errorf("doctor does not name the authenticated user:\n%s", output)
	}

	if strings.Contains(output, "jira-token-for-tests") {
		t.Errorf("doctor printed the token:\n%s", output)
	}
}

func TestDoctorOnlineNamesATokenSourceWithoutShowingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)

	const secret = "secret-from-the-command-1234"
	clitest.WriteFile(
		t, dir, `{"jira": {"base_url": "`+server.URL+`", "token_command": "echo `+secret+`"}, `+clitest.SlackWebhook+`}`,
	)

	// Act
	output, err := clitest.RunAtHome(t, dir, "doctor", "--online")

	// Assert
	if err != nil || !reached.Load() {
		t.Fatalf("doctor --online = %v, reached Jira %v:\n%s", err, reached.Load(), output)
	}

	if !strings.Contains(output, "token from token_command") {
		t.Errorf("doctor does not name where the token came from:\n%s", output)
	}

	if strings.Contains(output, secret) {
		t.Errorf("doctor printed the resolved token:\n%s", output)
	}
}

func TestDoctorOnlineSaysTheKeychainGaveNoTokenWithoutAskingJira(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+server.URL+`", "keychain": true}, `+clitest.SlackWebhook+`}`)

	// Act
	output, err := clitest.RunAtHome(t, dir, "doctor", "--online")

	// Assert
	if err == nil || reached.Load() {
		t.Fatalf("doctor --online = %v, reached Jira %v; want the token missing, Jira unasked:\n%s",
			err, reached.Load(), output)
	}

	if !strings.Contains(output, "keychain") {
		t.Errorf("doctor does not say the keychain gave no token:\n%s", output)
	}
}

func TestDoctorOnlineFailsOnARejectedCredential(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := clitest.JiraServer(t, http.StatusUnauthorized, "<html>login</html>", &reached)
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || !reached.Load() {
		t.Fatalf("doctor --online = %v, reached Jira %v; want the rejection:\n%s", err, reached.Load(), output)
	}

	wantExit(t, err, 3)

	if !strings.Contains(output, "not accepted") {
		t.Errorf("doctor does not explain the rejection:\n%s", output)
	}

	if strings.Contains(output, "jira-token-for-tests") {
		t.Errorf("doctor printed the token:\n%s", output)
	}
}

func TestDoctorOnlineFallsBackToTheLoginName(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	// An instance can be configured to withhold display names.
	server := clitest.JiraServer(t, http.StatusOK, `{"name":"fred","active":true}`, &reached)
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err != nil || !reached.Load() || !strings.Contains(output, "authenticates as fred (token from") {
		t.Errorf("doctor --online = %v, reached Jira %v; want the login name alone:\n%s", err, reached.Load(), output)
	}
}

// Nothing is wrong with the configuration — there is simply nothing to ask,
// because the only way to test a webhook is to post into someone's channel.
func TestDoctorOnlineSaysAWebhookCannotBeChecked(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		command string
		says    func(t *testing.T, output string) bool
	}{
		"the prose says why": {command: "doctor --online", says: func(_ *testing.T, output string) bool {
			return strings.Contains(output, "cannot be checked")
		}},
		"the JSON calls it unchecked": {command: "doctor --json --online", says: func(t *testing.T, output string) bool {
			t.Helper()

			return credentialStatusIn(decodeReport(t, output), messagingService) == uncheckedStatus
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var reached atomic.Bool

			dir := t.TempDir()
			server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
			clitest.WriteConfigFor(t, dir, server.URL)

			// Act
			output, err := clitest.Run(t, dir, strings.Fields(tt.command)...)

			// Assert
			if err != nil || !reached.Load() {
				t.Fatalf("%s failed on a webhook it merely cannot check: %v (%s)", tt.command, err, output)
			}

			if !tt.says(t, output) {
				t.Errorf("%s does not say the webhook went unchecked:\n%s", tt.command, output)
			}
		})
	}
}

func TestDoctorOnlineFailsWhenSlackIsNotConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+server.URL+`", "token": "jira-token-for-tests"}}`)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil {
		t.Fatalf("doctor --online accepted a missing Slack credential:\n%s", output)
	}

	if !strings.Contains(output, "messaging has no credential") {
		t.Errorf("doctor does not say Slack has no credential:\n%s", output)
	}
}

// pathWithOnlyGit points PATH at the test's own programs alone, with git among
// them, so the forge CLI is absent, unless the test installs one, while doctor
// can still read the repository. Emptying PATH
// outright would take git with it, and then there is no remote to resolve a
// token for in the first place.
func pathWithOnlyGit(t *testing.T) {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("these tests need git: %v", err)
	}

	err = os.Symlink(gitPath, filepath.Join(clitest.ProgramsOf(t), "git"))
	if err != nil {
		t.Fatalf("linking git: %v", err)
	}

	clitest.SetVariable(t, "PATH", clitest.ProgramsOf(t))
}

// unreachableForge is a remote on a host under .invalid, which RFC 6761 reserves
// so it can never resolve. A github.com remote would send the suite to the real
// api.github.com with a fake token — passing only because the token source still
// appears in the 401 line, and failing on a plane. This fails in milliseconds
// everywhere, and forge.kind and forge.host are what let doctor derive an API
// for it at all.
const unreachableForge = "git@" + unreachableHost + ":owner/repo.git"

// unreachableHost is the host of unreachableForge.
const unreachableHost = "forge.invalid"

// repoWithConfig makes a repository with an origin, and a configuration in it
// naming a local Jira and a forge token.
func repoWithConfig(t *testing.T, forgeToken string) string {
	t.Helper()

	dir := clitest.RepoWithRemote(t, unreachableForge)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}, `+clitest.SlackWebhook+
		`,`+
		` "forge": {"kind": "github", "host": "`+unreachableHost+`", "token": "`+forgeToken+`"}}`)

	return dir
}

func TestDoctorOnlineNamesWhereTheForgeTokenCameFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.SetVariable(t, "GH_ENTERPRISE_TOKEN", "forge-token-for-tests")
	clitest.SetVariable(t, "GH_HOST", unreachableHost)

	dir := repoWithConfig(t, "")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	// The forge cannot be reached, so the check fails — and still says where the
	// token it tried came from.
	if err == nil || !strings.Contains(output, "token from the environment") {
		t.Errorf("doctor --online = %v, want the unreachable forge reported with the token's source:\n%s", err, output)
	}

	// The source is the useful part. The token itself never is.
	if strings.Contains(output, "forge-token-for-tests") {
		t.Errorf("doctor printed the forge token:\n%s", output)
	}
}

func TestDoctorOnlineOffersGitHubsOwnVariableOnlyToGitHubsOwnHosts(t *testing.T) {
	t.Parallel()

	// Arrange
	pathWithOnlyGit(t)
	clitest.SetVariable(t, "GITHUB_TOKEN", "forge-token-for-tests")

	dir := repoWithConfig(t, "")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || strings.Contains(output, "token from the environment") {
		t.Errorf("doctor --online = %v, want no token found for %s:\n%s", err, unreachableHost, output)
	}
}

func TestDoctorOnlinePrefersTheEnvironmentOverTheConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.SetVariable(t, "GH_ENTERPRISE_TOKEN", "from-the-environment")
	clitest.SetVariable(t, "GH_HOST", unreachableHost)

	dir := repoWithConfig(t, "from-the-file")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || !strings.Contains(output, "token from the environment") {
		t.Errorf("doctor --online = %v, want the environment's token tried first:\n%s", err, output)
	}
}

func TestDoctorOnlineFallsBackToTheConfiguredForgeToken(t *testing.T) {
	t.Parallel()

	// Arrange
	// Without gh on PATH the configuration is the last source standing.
	pathWithOnlyGit(t)

	dir := repoWithConfig(t, "from-the-file")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || !strings.Contains(output, "token from forge.token") {
		t.Errorf("doctor --online = %v, want the configured token tried:\n%s", err, output)
	}
}

func TestDoctorOnlineReportsNoForgeTokenAtAll(t *testing.T) {
	t.Parallel()

	// Arrange
	pathWithOnlyGit(t)

	dir := repoWithConfig(t, "")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil {
		t.Fatalf("doctor accepted a missing forge token:\n%s", output)
	}

	// Naming all three sources is the point: the reader should not have to go
	// looking for which one they were supposed to use.
	for _, want := range []string{"GH_ENTERPRISE_TOKEN", "GH_HOST", "gh auth login", "forge.token"} {
		if !strings.Contains(output, want) {
			t.Errorf("doctor does not mention %q:\n%s", want, output)
		}
	}
}

func TestDoctorOnlineSaysThereIsNoForgeWithoutARemote(t *testing.T) {
	t.Parallel()

	// Arrange

	dir := t.TempDir()
	clitest.GitInit(t, dir)
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err != nil || !strings.Contains(output, "no repository remote") {
		t.Errorf("doctor --online = %v, want success explaining the absent forge:\n%s", err, output)
	}
}

func TestDoctorOnlineAsksForForgeKindOnAnUnknownHost(t *testing.T) {
	t.Parallel()

	// Arrange

	dir := clitest.RepoWithRemote(t, unreachableForge)
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	// A GitHub Enterprise Server and a self-managed GitLab are indistinguishable
	// from the remote, so the answer is to say which one — not to guess.
	if err != nil || !strings.Contains(output, "set forge.kind") {
		t.Errorf("doctor --online = %v, want it to say how to name an on-premises forge:\n%s", err, output)
	}
}

func TestDoctorOnlineRejectsAnUnknownForgeKind(t *testing.T) {
	t.Parallel()

	// Arrange

	dir := clitest.RepoWithRemote(t, unreachableForge)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}, `+clitest.SlackWebhook+
		`,`+
		` "forge": {"kind": "bitbucket"}}`)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || !strings.Contains(output, "bitbucket") {
		t.Errorf("doctor --online = %v, want forge.kind = bitbucket refused by name:\n%s", err, output)
	}
}

func TestDoctorOnlineDistinguishesAnUnreachableServiceFromARejection(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.invalid", "token": "t"}, `+clitest.SlackWebhook+`}`)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil {
		t.Fatalf("doctor --online accepted an unreachable service:\n%s", output)
	}

	if strings.Contains(err.Error(), "a credential was rejected") {
		t.Errorf("doctor reported an unreachable service as a rejected credential: %v", err)
	}

	if !strings.Contains(err.Error(), "could not be reached") {
		t.Errorf("doctor does not report the service as unreachable: %v", err)
	}

	wantExit(t, err, 5)
}

func TestDoctorOnlineCountsARedirectAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	// A sign-in gateway in front of Jira answers with a redirect, which is
	// refused: the credential was never put to Jira, so it was not rejected.
	server := httptest.NewServer(http.HandlerFunc(clitest.Redirecting))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || strings.Contains(err.Error(), "a credential was rejected") ||
		!strings.Contains(output, "redirect") {
		t.Errorf("doctor --online = %v, want the redirect told as unreachable, not rejected:\n%s", err, output)
	}

	wantExit(t, err, 5)
}

// askingToWait answers every request with a 429 that asks for thirty seconds.
func askingToWait(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Retry-After", "30")
	writer.WriteHeader(http.StatusTooManyRequests)
}

func TestDoctorOnlineCountsARateLimitAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	// Jira asks the caller to wait, which says nothing of the credential.
	server := httptest.NewServer(http.HandlerFunc(askingToWait))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online")

	// Assert
	if err == nil || strings.Contains(err.Error(), "a credential was rejected") ||
		!strings.Contains(output, "rate limited") {
		t.Errorf("doctor --online = %v, want the rate limit told as unreachable, not rejected:\n%s", err, output)
	}

	wantExit(t, err, 5)
}
