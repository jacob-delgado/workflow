// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// fieldValue is the value doctor printed for a label, as in "Forge:   GitHub…",
// or empty when it printed no such line. Reading one line, rather than the
// whole report, is what stops a check from passing on text some other section
// happens to print.
func fieldValue(output, label string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if value, found := strings.CutPrefix(line, label+":"); found {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func TestDoctorReportsAMissingFile(t *testing.T) {
	t.Parallel()

	// Act
	output, err := run(t, t.TempDir(), "doctor")

	// Assert
	if err == nil || !strings.Contains(output, "config init") {
		t.Errorf("doctor = %v, want an error saying how to create a config:\n%s", err, output)
	}

	wantExit(t, err, 3)
}

func TestDoctorNamesMissingFields(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if err == nil {
		t.Fatalf("expected an error for an incomplete config, got none (%s)", output)
	}

	wantExit(t, err, 3)

	if !strings.Contains(output, "messaging.client_id (workflow slack login) or messaging.webhook_url") {
		t.Errorf("doctor does not offer both Slack transports:\n%s", output)
	}

	// With no Slack credential at all, also demanding slack.channel would read
	// as "set three things" when either transport is the actual next step.
	if strings.Contains(output, "- messaging.channel") {
		t.Errorf("doctor asked for a channel before a credential:\n%s", output)
	}
}

func TestDoctorAcceptsACompleteConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if err != nil || !strings.Contains(output, "bearer token") {
		t.Errorf("doctor = %v, want success reporting the auth mode:\n%s", err, output)
	}
}

func TestDoctorReportsAConflictingKeyOverride(t *testing.T) {
	t.Parallel()

	// Arrange
	// commit and stage-all both live on the Commits pane, so binding commit to
	// stage-all's key is a conflict doctor should catch.
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"},`+
		` "ui": {"keys": {"commit": "a"}}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if err == nil {
		t.Fatalf("doctor accepted a conflicting ui.keys map:\n%s", output)
	}

	if !strings.Contains(output, "commit") || !strings.Contains(output, "stage-all") {
		t.Errorf("doctor does not report the key conflict:\n%s", output)
	}
}

func TestDoctorFailsOnAConfigurationOthersCanRead(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)

	err := os.Chmod(path, 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	// It says what is wrong and the one command that puts it right.
	if err == nil || !strings.Contains(output, "0644") || !strings.Contains(output, "chmod 600 "+path) {
		t.Errorf("doctor = %v, want it to refuse a file others can read and say how to fix it:\n%s", err, output)
	}

	wantExit(t, err, 3)
}

func TestDoctorReportsTheExternalTooling(t *testing.T) {
	t.Parallel()

	// Act
	output, _ := run(t, t.TempDir(), "doctor")

	// Assert
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

func TestDoctorFailsWhenRequiredToolingIsMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	// An empty PATH is the only portable way to make every program unfindable,
	// which is what proves the required/optional distinction actually bites.
	setVariable(t, "PATH", "")

	// Act
	output, err := run(t, t.TempDir(), "doctor")

	// Assert
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
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, "{not json")

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Fatalf("doctor = %v, want the malformed configuration reported:\n%s", err, output)
	}

	// A file that exists but cannot be parsed is a different problem from no
	// file at all, and telling someone to create one they already have is worse
	// than saying nothing.
	if strings.Contains(output, "config init") {
		t.Errorf("doctor told the user to create a file that already exists:\n%s", output)
	}
}

func TestDoctorSaysItCannotReadTheConfiguration(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root opens a file whatever its mode, so the sealed one would be read")
	}

	dir := t.TempDir()
	path := writeFile(t, dir, `{}`)

	err := os.Chmod(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	if got := fieldValue(output, "Configuration"); !strings.Contains(got, "cannot be read") {
		t.Errorf("doctor's Configuration line = %q, want it to say the file cannot be read:\n%s", got, output)
	}

	if !strings.Contains(output, "permission denied") {
		t.Errorf("doctor does not say why the configuration cannot be read:\n%s", output)
	}

	wantExit(t, err, 1)
}

func TestDoctorAcceptsAWebhookWithoutAChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	const webhook = "https://hooks.slack.com/services/T00000000/B00000000/supersecretpayload"

	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "`+webhook+`"}}`)

	// Act
	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor rejected a webhook-only configuration: %v (%s)", err, output)
	}

	// Assert
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

func TestDoctorDoesNotTouchTheNetworkWithoutTheFlag(t *testing.T) {
	t.Parallel()

	// Arrange
	var reached atomic.Bool

	dir := t.TempDir()
	server := jiraServer(t, http.StatusOK, jiraFixture, &reached)
	writeConfigFor(t, dir, server.URL)

	// Act
	output, err := run(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v (%s)", err, output)
	}

	// Assert
	if reached.Load() {
		t.Errorf("doctor called Jira without --online:\n%s", output)
	}

	if !strings.Contains(output, "--online") {
		t.Errorf("doctor does not say how to check the credentials:\n%s", output)
	}
}

// invalidValuesConfig misses nothing but fills two values in wrong: an
// insecure webhook and a key override that collides. Each still loads, so only
// doctor's review of the loaded file can catch them — in both of its reports.
const invalidValuesConfig = `{"jira":{"base_url":"https://jira.example.com","token":"t"},` +
	`"messaging":{"webhook_url":"http://hooks.example.com/x"},"ui":{"keys":{"commit":"a"}}}`

func TestDoctorReportsSetButInvalidValues(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, invalidValuesConfig)

	// Act
	out, err := run(t, dir, "doctor")

	// Assert
	if err == nil {
		t.Errorf("doctor passed a configuration with invalid values:\n%s", out)
	}

	wantExit(t, err, 3)

	for _, want := range []string{"Problems", "messaging.webhook_url", "stage-all"} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor did not flag %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "Missing") {
		t.Errorf("doctor reported a missing field in a complete configuration:\n%s", out)
	}
}
