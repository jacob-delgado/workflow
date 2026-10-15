// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestRequestLogReportsAFileItCannotOpen(t *testing.T) {
	t.Parallel()

	// --request-log names a file whose parent directory does not exist, so opening the
	// log fails before anything is asked of a service. The root opens it before
	// it reaches the terminal, which is what makes the interface's case
	// reachable at all.
	cases := [][]string{
		nil,
		strings.Fields("status"),
		strings.Fields("status ."),
		strings.Fields("doctor"),
		strings.Fields("config init"),
	}

	for _, args := range cases {
		name := strings.Join(append([]string{"workflow"}, args...), " ")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			logPath := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

			// Act
			_, err := clitest.Run(t, t.TempDir(), append([]string{"--request-log", logPath}, args...)...)

			// Assert
			// The error must name the request log: bare `workflow` also fails when
			// it cannot open a TTY, and status outside a repository fails anyway,
			// so a plain "an error occurred" check would pass whatever the cause.
			if err == nil || !strings.Contains(err.Error(), "request log") {
				t.Errorf("%s = %v, want it to fail opening the log in a missing directory", name, err)
			}
		})
	}
}

func TestLogReachesASubcommand(t *testing.T) {
	t.Parallel()

	// Bare status reads the directory it runs in; status DIR... wires each
	// directory it is given on its own, and must log those requests too.
	cases := [][]string{
		strings.Fields("status"),
		strings.Fields("status ."),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			clitest.FakeGh(t, clitest.GhResponses{Pulls: clitest.OpenPull("Add login"), Status: clitest.PassingStatus()})
			repo := statusFeatureRepo(t, server.URL)
			logPath := filepath.Join(t.TempDir(), "requests.log")

			// Act
			output, err := clitest.Run(t, repo, append([]string{"--request-log", logPath}, args...)...)
			if err != nil {
				t.Fatalf("%s: %v (%s)", name, err, output)
			}

			// Assert
			// status reads the issue from Jira and the pull request through gh;
			// both requests are outlined for a bug report.
			logged := clitest.ReadLog(t, logPath)
			if !strings.Contains(logged, "jira  GET") || !strings.Contains(logged, "forge GET") {
				t.Errorf("the request log does not outline %s's requests:\n%s", name, logged)
			}
		})
	}
}

func TestLogReachesDoctorOnline(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := clitest.Run(t, dir, "doctor", "--online", "--request-log", logPath)
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	// Assert
	// doctor --online is what a bug report asks for first, so its questions to
	// each service are what the log most needs to hold.
	if logged := clitest.ReadLog(t, logPath); !strings.Contains(logged, "jira  GET  /rest/api/2/myself") {
		t.Errorf("the request log does not outline doctor's check of Jira:\n%s", logged)
	}
}

func TestLogReachesTheGuidedInitsCheck(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	prompt := clitest.Scripted([]string{clitest.WorkingJira(t)}, []string{guidedToken})
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := clitest.RunGuided(t, dir, prompt, "--request-log", logPath, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	if logged := clitest.ReadLog(t, logPath); !strings.Contains(logged, "jira  GET  /rest/api/2/myself") {
		t.Errorf("the request log does not outline config init's check of Jira:\n%s", logged)
	}
}

func TestDryRunIsAPersistentFlag(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	// The flag comes before the subcommand, as it does for the interface.
	output, err := clitest.Run(t, repo, "--dry-run", "pr")

	// Assert
	if err != nil || !strings.Contains(output, "dry run: would push fix/PROJ-2-thing to origin and open") {
		t.Errorf("workflow --dry-run pr = %v, want the dry run previewed:\n%s", err, output)
	}
}

func TestDryRunReachesConfigInit(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	output, err := clitest.Run(t, dir, "--dry-run", "config", "init", "--template")

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, ".workflow.json"))
	if err != nil || !os.IsNotExist(statErr) || !strings.Contains(output, "dry run: would write") {
		t.Errorf("workflow --dry-run config init = %v, file %v, want nothing written:\n%s", err, statErr, output)
	}
}

func TestLogReachesSlackLogin(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeSlack(t, map[string]clitest.SlackAnswer{slackRefresh: {Status: http.StatusOK, Body: slackRenewed}})

	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"messaging": {"kind": "slack", "client_id": "`+clitest.SlackClientID+`",`+
		` "client_secret": "client-secret-old", "channel": "#dev"}}`)

	logPath := filepath.Join(t.TempDir(), "requests.log")
	prompt := asking([]string{clitest.SlackClientID}, typedLogin(), new([]string), new([]string))

	// Act
	output, err := clitest.RunGuided(t, dir, prompt, "--request-log", logPath, "slack", "login")
	if err != nil {
		t.Fatalf("slack login: %v (%s)", err, output)
	}

	// Assert
	logged := clitest.ReadLog(t, logPath)
	if !strings.Contains(logged, "slack POST "+slackRefresh) ||
		!strings.Contains(logged, "slack POST "+clitest.SlackAuthTest) {
		t.Errorf("the request log does not outline the login's refresh and its auth.test:\n%s", logged)
	}
}
