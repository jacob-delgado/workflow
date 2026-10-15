// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// jiraSecret is the token the JSON report must carry knowledge of without ever
// echoing.
const jiraSecret = "jira-token-super-secret"

// decodeReport parses doctor's JSON, failing the test when it is not valid JSON.
// The JSON leads output, and stderr's error, when doctor ended with one,
// follows it.
func decodeReport(t *testing.T, output string) map[string]any {
	t.Helper()

	var report map[string]any

	err := json.NewDecoder(strings.NewReader(output)).Decode(&report)
	if err != nil {
		t.Fatalf("doctor --json is not valid JSON: %v\n%s", err, output)
	}

	return report
}

func TestDoctorJSONReportsTheFactsAsData(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(
		t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "`+jiraSecret+`"}, `+clitest.SlackWebhook+`}`,
	)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json")

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
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, invalidValuesConfig)

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--json")

	// Assert
	wantExit(t, err, 3)

	configuration, _ := decodeReport(t, output)["configuration"].(map[string]any)
	problems, _ := configuration["problems"].([]any)

	if missed := unmentioned(problems, "messaging.webhook_url", "stage-all"); missed != "" {
		t.Errorf("configuration.problems = %v, want an entry naming %q:\n%s", problems, missed, output)
	}
}

func TestDoctorJSONNeverEchoesASecret(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(
		t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "`+jiraSecret+`"}, `+clitest.SlackWebhook+`}`,
	)

	// Act
	output, _ := clitest.Run(t, dir, "doctor", "--json")

	// Assert
	decodeReport(t, output) // it is JSON, so the check is not fooled by prose

	if strings.Contains(output, jiraSecret) || strings.Contains(output, "not-real") {
		t.Errorf("the report leaked a credential:\n%s", output)
	}
}

func TestDoctorJSONReportsAMissingFileAsData(t *testing.T) {
	t.Parallel()

	// Act
	output, err := clitest.Run(t, t.TempDir(), "doctor", "--json")

	// Assert
	report := decodeReport(t, output)
	if err == nil || report["config_problem"] == nil {
		t.Errorf("doctor --json = %v, want a config problem in the data:\n%s", err, output)
	}
}

func TestDoctorJSONSaysWhyThereIsNoRepository(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		path string
		want string
	}{
		"git not on PATH":     {path: "", want: "git is not on PATH"},
		"outside a work tree": {path: os.Getenv("PATH"), want: "is not in a git work tree"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clitest.SetVariable(t, "PATH", tt.path)

			// Act
			printed, _ := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "doctor", "--json")

			// Assert
			repository, _ := decodeReport(t, printed.Stdout)["repository"].(map[string]any)
			if problem, _ := repository["problem"].(string); !strings.Contains(problem, tt.want) ||
				repository["inside_work_tree"] != false {
				t.Errorf("repository = %v, want it outside a work tree because %q", repository, tt.want)
			}
		})
	}
}
