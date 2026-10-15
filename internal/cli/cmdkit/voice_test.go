// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

func TestQuietAndVerboseTogetherAreAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--quiet", "--verbose", "doctor")

	// Assert
	wantExit(t, err, 2)

	if printed.Stdout != "" || !strings.Contains(printed.Stderr, "--quiet and --verbose") {
		t.Errorf("stdout %q, stderr %q; want nothing run and the two flags named", printed.Stdout, printed.Stderr)
	}
}

func TestALogFormatItDoesNotKnowIsAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--log-format", "xml", "doctor")

	// Assert
	wantExit(t, err, 2)

	if !strings.Contains(printed.Stderr, "text, json or logfmt") {
		t.Errorf("stderr = %q, want the formats --log-format takes", printed.Stderr)
	}
}

func TestQuietDropsANoteAndKeepsTheArtifact(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{}`)

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "-q", "config", "show")

	// Assert
	if err != nil || printed.Stderr != "" || !strings.HasPrefix(printed.Stdout, "{") {
		t.Errorf("config show -q = %v, stdout %q, stderr %q; want the JSON alone", err, printed.Stdout, printed.Stderr)
	}
}

func TestJSONLogFormatSaysEachNoteAndTheErrorAsObjects(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--log-format", "json", "launch")

	// Assert
	wantExit(t, err, 2)

	var said map[string]any

	decodeErr := json.Unmarshal([]byte(printed.Stderr), &said)
	if decodeErr != nil || said["level"] != "error" || said["exit_status"] != 2.0 ||
		!strings.HasPrefix(fmt.Sprint(said["msg"]), `unknown command "launch"`) {
		t.Errorf("stderr = %q (%v), want one object: the error, its message and exit status 2", printed.Stderr, decodeErr)
	}
}

func TestVerboseSaysEachRequestAsADebugLine(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "--verbose", "doctor", "--online")

	// Assert
	if err != nil || !strings.Contains(printed.Stderr, "DEBUG jira GET /rest/api/2/myself 200 ") {
		t.Errorf("doctor --online --verbose = %v, stderr %q; want Jira's request as a debug line", err, printed.Stderr)
	}

	if strings.Contains(printed.Stdout, "DEBUG") {
		t.Errorf("stdout = %q, want the report alone", printed.Stdout)
	}
}

func TestVerboseKeepsTheRequestLogItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	printed, err := clitest.RunStreams(
		t, dir, clitest.UnusedPrompt(t), "--verbose", "--request-log", logPath, "doctor", "--online",
	)

	// Assert
	logged := clitest.ReadLog(t, logPath)
	if err != nil || !strings.Contains(logged, "jira  GET  /rest/api/2/myself") ||
		!strings.Contains(printed.Stderr, "DEBUG jira GET /rest/api/2/myself") {
		t.Errorf("doctor = %v, logged %q, stderr %q; want the request in the file and on stderr", err, logged, printed.Stderr)
	}
}

func TestTheOldLogFlagIsAUsageErrorNamingItsNewName(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--log", "requests.log", "doctor")

	// Assert
	wantExit(t, err, 2)

	if printed.Stdout != "" || !strings.Contains(printed.Stderr, "--request-log") {
		t.Errorf("stdout %q, stderr %q; want nothing run, and the flag's new name", printed.Stdout, printed.Stderr)
	}
}

func TestTheOldLogFlagIsNotInTheHelp(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--help")

	// Assert
	if err != nil || strings.Contains(printed.Stdout, "--log ") || !strings.Contains(printed.Stdout, "--request-log") {
		t.Errorf("--help = %v:\n%s\nwant --request-log alone", err, printed.Stdout)
	}
}
