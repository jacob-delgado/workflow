// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// `workflow doctor --json --online`: each credential's check as data.

func TestDoctorJSONOnlineNamesTheIdentityWithoutTheToken(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
	defer server.Close()

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json", "--online")

	// Assert
	if strings.Contains(output, "jira-token-for-tests") {
		t.Errorf("the online report leaked the token:\n%s", output)
	}

	report := decodeReport(t, output)

	credentials, ok := report["credentials"].(map[string]any)

	if !ok || credentials["checked"] != true || !strings.Contains(output, "authenticates as") {
		t.Errorf("the online report does not name the identity as data:\n%s", output)
	}
}

// The services the online report names, as its results carry them.
const (
	jiraService      = "jira"
	messagingService = "slack"
)

// The statuses the online report gives a check doctor could not make, and a
// service that never answered.
const (
	uncheckedStatus   = "unchecked"
	unreachableStatus = "unreachable"
	missingStatus     = "missing"
)

// credentialStatusIn is the status the online report gives service, or "" when
// the report names no such service.
func credentialStatusIn(report map[string]any, service string) string {
	credentials, _ := report["credentials"].(map[string]any)
	results, _ := credentials["results"].([]any)

	for _, result := range results {
		line, _ := result.(map[string]any)
		if line["service"] == service {
			status, _ := line["status"].(string)

			return status
		}
	}

	return ""
}

func TestDoctorJSONOnlineReportsARedirectAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(clitest.Redirecting))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json", "--online")

	// Assert
	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != unreachableStatus {
		t.Errorf("the online report calls Jira's redirect %q, want unreachable:\n%s", got, output)
	}
}

func TestDoctorJSONOnlineChecksNothingWhenTheFileDidNotLoad(t *testing.T) {
	t.Parallel()

	// Arrange
	// Every forge variable is cleared, so only gh could answer for the forge:
	// a token in the environment would resolve first and gh would never run.
	dir := clitest.RepoWithRemote(t, githubSSHRemote)
	clitest.WriteFile(t, dir, "{not json")
	ghRan := ghSignedOutRecording(t)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 3)

	credentials, _ := decodeReport(t, output)["credentials"].(map[string]any)
	if credentials["checked"] != false || credentials["results"] != nil {
		t.Errorf("credentials = %v, want none checked over a file that did not load:\n%s", credentials, output)
	}

	_, statErr := os.Stat(ghRan)
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("doctor ran gh over a file that did not load (stat: %v)", statErr)
	}
}

func TestDoctorJSONOnlineReportsARateLimitAsUnreachable(t *testing.T) {
	t.Parallel()

	// Arrange
	server := httptest.NewServer(http.HandlerFunc(askingToWait))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--json", "--online")

	// Assert
	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != unreachableStatus {
		t.Errorf("the online report calls Jira's rate limit %q, want unreachable:\n%s", got, output)
	}

	if !strings.Contains(output, "30s") {
		t.Errorf("the online report does not name the wait Jira asked for:\n%s", output)
	}

	wantExit(t, err, 5)
}

func TestDoctorJSONOnlineCallsARefusedCredentialRejected(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool))

	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--json", "--online")

	// Assert
	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != "rejected" {
		t.Errorf("the online report calls Jira's refusal %q, want rejected:\n%s", got, output)
	}

	wantExit(t, err, 3)
}

// configuredWith is a directory holding a configuration made of fields, for a
// test whose credential sits in the file rather than in a repository.
func configuredWith(t *testing.T, fields string) string {
	t.Helper()

	dir := t.TempDir()
	clitest.WriteFile(t, dir, "{"+fields+"}")

	return dir
}

func TestDoctorJSONOnlineCallsAnAbsentCredentialMissing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		service string
		setup   func(t *testing.T) string
	}{
		"no jira.token": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`"}, `+clitest.SlackWebhook)
		}},
		"a jira token_command that fails": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(
				t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token_command": "false"}, `+clitest.SlackWebhook,
			)
		}},
		"a jira token_command that prints nothing": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(
				t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token_command": "true"}, `+clitest.SlackWebhook,
			)
		}},
		"no messaging credential": {service: messagingService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}`)
		}},
		// The file keeps the token's credentials, so no keychain is read.
		"a Slack user token not logged in": {service: messagingService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}, `+
				`"messaging": {"client_id": "1234.5678", "client_secret": "client-secret-9999", "channel": "#dev"}`)
		}},
		"no forge token": {service: "forge", setup: func(t *testing.T) string {
			t.Helper()
			pathWithOnlyGit(t)

			dir := clitest.RepoWithRemote(t, githubSSHRemote)
			clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

			return dir
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := tt.setup(t)

			// Act
			output, err := clitest.RunAtHome(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, 3)

			if got := credentialStatusIn(decodeReport(t, output), tt.service); got != missingStatus {
				t.Errorf("the online report calls %s's absent credential %q, want missing:\n%s", tt.service, got, output)
			}

			if err != nil && strings.Contains(err.Error(), "rejected") {
				t.Errorf("doctor --json --online = %v, want nothing called rejected when nothing was asked", err)
			}
		})
	}
}

func TestDoctorJSONOnlineAsksJiraNothingWhenItsTokenEnvIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, &reached)
	clitest.SetVariable(t, clitest.EmptyTokenVariable, "")
	dir := configuredWith(t,
		`"jira": {"base_url": "`+server.URL+`", "token_env": "`+clitest.EmptyTokenVariable+`"}, `+clitest.SlackWebhook)

	// Act
	output, err := clitest.RunAtHome(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 3)

	if reached.Load() {
		t.Errorf("doctor asked Jira with the empty token %s gave it", clitest.EmptyTokenVariable)
	}

	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != "missing" {
		t.Errorf("the online report calls an empty %s %q, want missing:\n%s", clitest.EmptyTokenVariable, got, output)
	}
}

// forgeKindRepository is a repository on a host that names neither forge, with
// a configuration whose forge block is forgeFields.
func forgeKindRepository(t *testing.T, forgeFields string) string {
	t.Helper()

	dir := clitest.RepoWithRemote(t, unreachableForge)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token": "t"}, `+clitest.SlackWebhook+
		`,`+
		` "forge": {`+forgeFields+`}}`)

	return dir
}

func TestDoctorJSONOnlineCallsAForgeItCannotAskUnchecked(t *testing.T) {
	t.Parallel()

	// A forge.kind that cannot be used still exits 3, through the configuration
	// section that names it; the forge's credential is not what is wrong.
	cases := map[string]struct {
		setup    func(t *testing.T) string
		wantExit int
	}{
		"no repository remote": {wantExit: 0, setup: func(t *testing.T) string {
			t.Helper()

			dir := t.TempDir()
			clitest.GitInit(t, dir)
			clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

			return dir
		}},
		"a host that names neither forge": {wantExit: 0, setup: func(t *testing.T) string {
			t.Helper()

			dir := clitest.RepoWithRemote(t, unreachableForge)
			clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

			return dir
		}},
		"a forge.kind with no forge.host": {wantExit: 3, setup: func(t *testing.T) string {
			t.Helper()

			return forgeKindRepository(t, `"kind": "github"`)
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := tt.setup(t)

			// Act
			output, err := clitest.Run(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, tt.wantExit)

			if got := credentialStatusIn(decodeReport(t, output), "forge"); got != uncheckedStatus {
				t.Errorf("the online report calls a forge it could not ask %q, want unchecked:\n%s", got, output)
			}
		})
	}
}

func TestDoctorOnlineSaysTheSameOfEachCredentialInProseAndJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.JiraFixture, new(atomic.Bool))
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, server.URL)

	prose, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--online")

	// Act
	asJSON, _ := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor", "--online", "--json")

	// Assert
	credentials, _ := decodeReport(t, asJSON.Stdout)["credentials"].(map[string]any)
	results, _ := credentials["results"].([]any)

	if len(results) != 3 {
		t.Fatalf("the online report has %d results, want Jira's, messaging's and the forge's:\n%s",
			len(results), asJSON.Stdout)
	}

	for _, result := range results {
		line, _ := result.(map[string]any)
		service, _ := line["service"].(string)
		detail, _ := line["detail"].(string)

		if said := fmt.Sprintf("  %-10s %s\n", service, detail); detail == "" || !strings.Contains(prose.Stdout, said) {
			t.Errorf("the JSON says %q of %s, which the prose report does not:\n%s", detail, service, prose.Stdout)
		}
	}
}
