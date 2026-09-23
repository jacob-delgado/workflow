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
)

func TestRequestLogReportsAFileItCannotOpen(t *testing.T) {
	// --log names a file whose parent directory does not exist, so opening the
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
			// Arrange
			logPath := filepath.Join(t.TempDir(), "missing-dir", "requests.log")

			// Act
			_, err := run(t, t.TempDir(), append([]string{"--log", logPath}, args...)...)

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

// readLog is what the request log at path holds, failing the test if it
// cannot be read.
func readLog(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the request log: %v", err)
	}

	return string(contents)
}

func TestLogReachesASubcommand(t *testing.T) {
	// Bare status reads the directory it runs in; status DIR... wires each
	// directory it is given on its own, and must log those requests too.
	cases := [][]string{
		strings.Fields("status"),
		strings.Fields("status ."),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			// Arrange
			server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "Fix login"), new(atomic.Bool))
			fakeGh(t, ghResponses{pulls: openPull("Add login"), status: passingStatus()})
			repo := statusFeatureRepo(t, server.URL)
			logPath := filepath.Join(t.TempDir(), "requests.log")

			// Act
			output, err := run(t, repo, append([]string{"--log", logPath}, args...)...)
			if err != nil {
				t.Fatalf("%s: %v (%s)", name, err, output)
			}

			// Assert
			// status reads the issue from Jira and the pull request through gh;
			// both requests are outlined for a bug report.
			logged := readLog(t, logPath)
			if !strings.Contains(logged, "jira  GET") || !strings.Contains(logged, "forge GET") {
				t.Errorf("the request log does not outline %s's requests:\n%s", name, logged)
			}
		})
	}
}

func TestLogReachesDoctorOnline(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeConfigFor(t, dir, workingJira(t))
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := run(t, dir, "doctor", "--online", "--log", logPath)
	if err != nil {
		t.Fatalf("doctor --online: %v (%s)", err, output)
	}

	// Assert
	// doctor --online is what a bug report asks for first, so its questions to
	// each service are what the log most needs to hold.
	if logged := readLog(t, logPath); !strings.Contains(logged, "jira  GET  /rest/api/2/myself") {
		t.Errorf("the request log does not outline doctor's check of Jira:\n%s", logged)
	}
}

func TestLogReachesTheGuidedInitsCheck(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	prompt := scripted([]string{workingJira(t)}, []string{guidedToken})
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	output, err := runGuided(t, dir, prompt, "--log", logPath, "config", "init")
	if err != nil {
		t.Fatalf("config init: %v (%s)", err, output)
	}

	// Assert
	if logged := readLog(t, logPath); !strings.Contains(logged, "jira  GET  /rest/api/2/myself") {
		t.Errorf("the request log does not outline config init's check of Jira:\n%s", logged)
	}
}

func TestDryRunIsAPersistentFlag(t *testing.T) {
	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	// The flag comes before the subcommand, as it does for the interface.
	output, err := run(t, repo, "--dry-run", "pr")

	// Assert
	if err != nil || !strings.Contains(output, "dry run: would push fix/PROJ-2-thing and open") {
		t.Errorf("workflow --dry-run pr = %v, want the dry run previewed:\n%s", err, output)
	}
}

func TestDryRunReachesConfigInit(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	output, err := run(t, dir, "--dry-run", "config", "init", "--template")

	// Assert
	_, statErr := os.Stat(filepath.Join(dir, ".workflow.json"))
	if err != nil || !os.IsNotExist(statErr) || !strings.Contains(output, "dry run: would write") {
		t.Errorf("workflow --dry-run config init = %v, file %v, want nothing written:\n%s", err, statErr, output)
	}
}
