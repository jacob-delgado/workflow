// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestAFailedCommandSaysItsErrorOnceTagged(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "launch")

	// Assert
	wantExit(t, err, 2)

	if strings.Count(printed.Stderr, "ERROR") != 1 ||
		!strings.HasPrefix(printed.Stderr, `ERROR unknown command "launch"`) {
		t.Errorf("stderr = %q, want the error said once, after its tag", printed.Stderr)
	}

	if strings.Contains(printed.Stderr, "workflow:") || printed.Stdout != "" {
		t.Errorf("stdout %q, stderr %q; want no workflow: prefix, and nothing on stdout", printed.Stdout, printed.Stderr)
	}
}

func TestASucceededCommandSaysNoError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := clitest.RunStreams(t, t.TempDir(), clitest.UnusedPrompt(t), "--version")

	// Assert
	if err != nil || strings.Contains(printed.Stderr, "ERROR") {
		t.Errorf("--version = %v, stderr %q; want no error said", err, printed.Stderr)
	}
}

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

func TestVerboseWebSaysTheServersRequestsAsDebugLines(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteConfigFor(t, dir, clitest.WorkingJira(t))

	askingJira := func(string) cli.RunWeb {
		return func(ctx context.Context, _ config.Config, deps webserver.Deps, _ webserver.Info, _ io.Writer) error {
			_, _ = deps.Jira.Issue(ctx, "PROJ-1")

			return nil
		}
	}

	var ran rootRun

	// Act
	_, stderr, err := executeRoot(
		t, clitest.Place{Dir: dir, Home: t.TempDir()}, ran.runInterface, askingJira, "--verbose", "--web",
	)

	// Assert
	if err != nil || !strings.Contains(stderr, "DEBUG jira GET /rest/api/2/issue/PROJ-1 ") {
		t.Errorf("workflow --verbose --web = %v, stderr %q; want the server's request as a debug line", err, stderr)
	}
}

func TestVerboseLeavesTheTerminalInterfaceToDrawAlone(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir(), "--verbose")

	// Assert
	if ran.err != nil || ran.interfaces != 1 || ran.stderr != "" {
		t.Errorf("workflow --verbose = %v after %d interfaces, stderr %q; want the interface opened, nothing said",
			ran.err, ran.interfaces, ran.stderr)
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
