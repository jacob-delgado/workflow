// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// jiraSecret is the token the JSON report must carry knowledge of without ever
// echoing.
const jiraSecret = "jira-token-super-secret"

// decodeReport parses doctor's JSON, failing the test when it is not valid JSON.
func decodeReport(t *testing.T, output string) map[string]any {
	t.Helper()

	var report map[string]any

	err := json.Unmarshal([]byte(output), &report)
	if err != nil {
		t.Fatalf("doctor --json is not valid JSON: %v\n%s", err, output)
	}

	return report
}

func TestDoctorJSONReportsTheFactsAsData(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "`+jiraSecret+`"}, `+slackWebhook+`}`)

	// Act
	output, _ := run(t, dir, "doctor", "--json")

	// Assert
	report := decodeReport(t, output)
	if report["version"] == nil {
		t.Errorf("the report has no version:\n%s", output)
	}

	configuration, ok := report["configuration"].(map[string]any)
	if !ok || configuration["jira_url"] != "https://jira.example.com" {
		t.Errorf("the report does not carry the Jira URL as data:\n%s", output)
	}

	// The messaging facts carry the service-neutral keys, not the old Slack ones.
	if configuration["messaging_mode"] != "incoming webhook" {
		t.Errorf("the report does not carry messaging_mode as data:\n%s", output)
	}

	if target, hasTarget := configuration["messaging_target"].(string); !hasTarget || target == "" {
		t.Errorf("the report does not carry messaging_target as data:\n%s", output)
	}

	if _, old := configuration["slack_mode"]; old {
		t.Errorf("the report still carries the retired slack_mode key:\n%s", output)
	}

	// Nothing is filled in wrong, so problems is present and null, as missing is.
	if problems, present := configuration["problems"]; !present || problems != nil {
		t.Errorf("configuration.problems = %v (present %t), want null:\n%s", problems, present, output)
	}
}

// unmentioned is the first of wants that no item in items mentions, or "" when
// every one is mentioned.
func unmentioned(items []any, wants ...string) string {
	for _, want := range wants {
		if !slices.ContainsFunc(items, func(item any) bool {
			text, isText := item.(string)

			return isText && strings.Contains(text, want)
		}) {
			return want
		}
	}

	return ""
}

func TestDoctorJSONFailsOnSetButInvalidValues(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, invalidValuesConfig)

	// Act
	output, err := run(t, dir, "doctor", "--json")

	// Assert
	wantExit(t, err, 3)

	configuration, _ := decodeReport(t, output)["configuration"].(map[string]any)
	problems, _ := configuration["problems"].([]any)

	if missed := unmentioned(problems, "jira.base_url", "messaging.webhook_url", "forge.kind", "stage-all"); missed != "" {
		t.Errorf("configuration.problems = %v, want an entry naming %q:\n%s", problems, missed, output)
	}
}

func TestDoctorJSONNeverEchoesASecret(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "`+jiraSecret+`"}, `+slackWebhook+`}`)

	// Act
	output, _ := run(t, dir, "doctor", "--json")

	// Assert
	decodeReport(t, output) // it is JSON, so the check is not fooled by prose

	if strings.Contains(output, jiraSecret) || strings.Contains(output, "not-real") {
		t.Errorf("the report leaked a credential:\n%s", output)
	}
}

func TestDoctorJSONReportsAMissingFileAsData(t *testing.T) {
	// Act
	output, err := run(t, t.TempDir(), "doctor", "--json")

	// Assert
	report := decodeReport(t, output)
	if err == nil || report["config_problem"] == nil {
		t.Errorf("doctor --json = %v, want a config problem in the data:\n%s", err, output)
	}
}

func TestDoctorJSONOnlineNamesTheIdentityWithoutTheToken(t *testing.T) {
	// Arrange
	var reached atomic.Bool

	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	defer server.Close()

	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output, _ := run(t, dir, "doctor", "--json", "--online")

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

// uncheckedStatus is the status the online report gives a check doctor could
// not make.
const uncheckedStatus = "unchecked"

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
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(redirecting))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output, _ := run(t, dir, "doctor", "--json", "--online")

	// Assert
	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != "unreachable" {
		t.Errorf("the online report calls Jira's redirect %q, want unreachable:\n%s", got, output)
	}
}

func TestDoctorJSONOnlineChecksNothingWhenTheFileDidNotLoad(t *testing.T) {
	// Arrange
	// Every forge variable is cleared, so only gh could answer for the forge:
	// a token in the environment would resolve first and gh would never run.
	clearForgeEnvironment(t)
	dir := repoWithRemote(t, githubSSHRemote)
	writeFile(t, dir, "{not json")
	ghRan := ghSignedOutRecording(t)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

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
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(askingToWait))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != "unreachable" {
		t.Errorf("the online report calls Jira's rate limit %q, want unreachable:\n%s", got, output)
	}

	if !strings.Contains(output, "30s") {
		t.Errorf("the online report does not name the wait Jira asked for:\n%s", output)
	}

	wantExit(t, err, 5)
}

// emptyTokenVariable is a token_env the tests set to nothing, standing in for a
// variable the shell never exported.
const emptyTokenVariable = "WORKFLOW_TEST_EMPTY_TOKEN"

// configuredWith is a directory holding a configuration made of fields, for a
// test whose credential sits in the file rather than in a repository.
func configuredWith(t *testing.T, fields string) string {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, dir, "{"+fields+"}")

	return dir
}

func TestDoctorJSONOnlineCallsAnAbsentCredentialMissing(t *testing.T) {
	cases := map[string]struct {
		service string
		setup   func(t *testing.T) string
	}{
		"no jira.token": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`"}, `+slackWebhook)
		}},
		"a jira token_command that fails": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`", "token_command": "false"}, `+slackWebhook)
		}},
		"a jira token_command that prints nothing": {service: jiraService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`", "token_command": "true"}, `+slackWebhook)
		}},
		"no messaging credential": {service: messagingService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}`)
		}},
		"a messaging token_command that fails": {service: messagingService, setup: func(t *testing.T) string {
			t.Helper()

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}, `+
				`"messaging": {"token_command": "false", "channel": "#dev"}`)
		}},
		"a messaging token_env naming an empty variable": {service: messagingService, setup: func(t *testing.T) string {
			t.Helper()
			t.Setenv(emptyTokenVariable, "")

			return configuredWith(t, `"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}, `+
				`"messaging": {"token_env": "`+emptyTokenVariable+`", "channel": "#dev"}`)
		}},
		"no forge token": {service: "forge", setup: func(t *testing.T) string {
			t.Helper()
			clearForgeEnvironment(t)
			pathWithOnlyGit(t)

			dir := repoWithRemote(t, githubSSHRemote)
			writeConfigFor(t, dir, workingJira(t))

			return dir
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := tt.setup(t)

			// Act
			output, err := run(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, 3)

			if got := credentialStatusIn(decodeReport(t, output), tt.service); got != "missing" {
				t.Errorf("the online report calls %s's absent credential %q, want missing:\n%s", tt.service, got, output)
			}

			if err != nil && strings.Contains(err.Error(), "rejected") {
				t.Errorf("doctor --json --online = %v, want nothing called rejected when nothing was asked", err)
			}
		})
	}
}

func TestDoctorJSONOnlineAsksJiraNothingWhenItsTokenEnvIsEmpty(t *testing.T) {
	// Arrange
	var reached atomic.Bool

	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	t.Setenv(emptyTokenVariable, "")
	dir := configuredWith(t,
		`"jira": {"base_url": "`+server.URL+`", "token_env": "`+emptyTokenVariable+`"}, `+slackWebhook)

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 3)

	if reached.Load() {
		t.Errorf("doctor asked Jira with the empty token %s gave it", emptyTokenVariable)
	}

	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != "missing" {
		t.Errorf("the online report calls an empty %s %q, want missing:\n%s", emptyTokenVariable, got, output)
	}
}

// forgeKindRepository is a repository on a host that names neither forge, with
// a configuration whose forge block is forgeFields.
func forgeKindRepository(t *testing.T, forgeFields string) string {
	t.Helper()

	dir := repoWithRemote(t, unreachableForge)
	writeFile(t, dir, `{"jira": {"base_url": "`+workingJira(t)+`", "token": "t"}, `+slackWebhook+`,`+
		` "forge": {`+forgeFields+`}}`)

	return dir
}

func TestDoctorJSONOnlineCallsAForgeItCannotAskUnchecked(t *testing.T) {
	// A forge.kind that cannot be used still exits 3, through the configuration
	// section that names it; the forge's credential is not what is wrong.
	cases := map[string]struct {
		setup    func(t *testing.T) string
		wantExit int
	}{
		"no repository remote": {wantExit: 0, setup: func(t *testing.T) string {
			t.Helper()

			dir := t.TempDir()
			gitInit(t, dir)
			writeConfigFor(t, dir, workingJira(t))

			return dir
		}},
		"a host that names neither forge": {wantExit: 0, setup: func(t *testing.T) string {
			t.Helper()

			dir := repoWithRemote(t, unreachableForge)
			writeConfigFor(t, dir, workingJira(t))

			return dir
		}},
		"a forge.kind naming no forge": {wantExit: 3, setup: func(t *testing.T) string {
			t.Helper()

			return forgeKindRepository(t, `"kind": "bitbucket", "host": "`+unreachableHost+`"`)
		}},
		"a forge.kind with no forge.host": {wantExit: 3, setup: func(t *testing.T) string {
			t.Helper()

			return forgeKindRepository(t, `"kind": "github"`)
		}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			clearForgeEnvironment(t)
			dir := tt.setup(t)

			// Act
			output, err := run(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, tt.wantExit)

			if got := credentialStatusIn(decodeReport(t, output), "forge"); got != uncheckedStatus {
				t.Errorf("the online report calls a forge it could not ask %q, want unchecked:\n%s", got, output)
			}
		})
	}
}

func TestDoctorJSONOnlineCallsJiraUncheckedAtAnAddressItCannotUse(t *testing.T) {
	// Arrange
	// The configuration section fails the address, so the run still exits 3;
	// Jira is never asked, so no credential is what is wrong.
	dir := t.TempDir()
	writeConfigFor(t, dir, "ftp://jira.example.com")

	// Act
	output, err := run(t, dir, "doctor", "--json", "--online")

	// Assert
	wantExit(t, err, 3)

	if got := credentialStatusIn(decodeReport(t, output), jiraService); got != uncheckedStatus {
		t.Errorf("the online report calls a Jira at an unusable address %q, want unchecked:\n%s", got, output)
	}

	if err != nil && strings.Contains(err.Error(), "a credential was rejected") {
		t.Errorf("doctor --json --online = %v, want no credential rejected when Jira was never asked", err)
	}
}
