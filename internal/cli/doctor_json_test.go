// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if got := credentialStatusIn(decodeReport(t, output), "jira"); got != "unreachable" {
		t.Errorf("the online report calls Jira's redirect %q, want unreachable:\n%s", got, output)
	}
}
