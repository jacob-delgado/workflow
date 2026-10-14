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
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

func TestAFailedCommandSaysItsErrorOnceTagged(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "launch")

	// Assert
	wantExit(t, err, 2)

	if strings.Count(printed.stderr, "ERROR") != 1 ||
		!strings.HasPrefix(printed.stderr, `ERROR unknown command "launch"`) {
		t.Errorf("stderr = %q, want the error said once, after its tag", printed.stderr)
	}

	if strings.Contains(printed.stderr, "workflow:") || printed.stdout != "" {
		t.Errorf("stdout %q, stderr %q; want no workflow: prefix, and nothing on stdout", printed.stdout, printed.stderr)
	}
}

func TestASucceededCommandSaysNoError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--version")

	// Assert
	if err != nil || strings.Contains(printed.stderr, "ERROR") {
		t.Errorf("--version = %v, stderr %q; want no error said", err, printed.stderr)
	}
}

func TestQuietAndVerboseTogetherAreAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--quiet", "--verbose", "doctor")

	// Assert
	wantExit(t, err, 2)

	if printed.stdout != "" || !strings.Contains(printed.stderr, "--quiet and --verbose") {
		t.Errorf("stdout %q, stderr %q; want nothing run and the two flags named", printed.stdout, printed.stderr)
	}
}

func TestALogFormatItDoesNotKnowIsAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--log-format", "xml", "doctor")

	// Assert
	wantExit(t, err, 2)

	if !strings.Contains(printed.stderr, "text, json or logfmt") {
		t.Errorf("stderr = %q, want the formats --log-format takes", printed.stderr)
	}
}

func TestQuietDropsANoteAndKeepsTheArtifact(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{}`)

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "-q", "config", "show")

	// Assert
	if err != nil || printed.stderr != "" || !strings.HasPrefix(printed.stdout, "{") {
		t.Errorf("config show -q = %v, stdout %q, stderr %q; want the JSON alone", err, printed.stdout, printed.stderr)
	}
}

func TestJSONLogFormatSaysEachNoteAndTheErrorAsObjects(t *testing.T) {
	t.Parallel()

	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "--log-format", "json", "launch")

	// Assert
	wantExit(t, err, 2)

	var said map[string]any

	decodeErr := json.Unmarshal([]byte(printed.stderr), &said)
	if decodeErr != nil || said["level"] != "error" || said["exit_status"] != 2.0 ||
		!strings.HasPrefix(fmt.Sprint(said["msg"]), `unknown command "launch"`) {
		t.Errorf("stderr = %q (%v), want one object: the error, its message and exit status 2", printed.stderr, decodeErr)
	}
}

func TestVerboseSaysEachRequestAsADebugLine(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeConfigFor(t, dir, workingJira(t))

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "--verbose", "doctor", "--online")

	// Assert
	if err != nil || !strings.Contains(printed.stderr, "DEBUG jira GET /rest/api/2/myself 200 ") {
		t.Errorf("doctor --online --verbose = %v, stderr %q; want Jira's request as a debug line", err, printed.stderr)
	}

	if strings.Contains(printed.stdout, "DEBUG") {
		t.Errorf("stdout = %q, want the report alone", printed.stdout)
	}
}

func TestVerboseKeepsTheRequestLogItIsGiven(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeConfigFor(t, dir, workingJira(t))
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "--verbose", "--log", logPath, "doctor", "--online")

	// Assert
	logged := readLog(t, logPath)
	if err != nil || !strings.Contains(logged, "jira  GET  /rest/api/2/myself") ||
		!strings.Contains(printed.stderr, "DEBUG jira GET /rest/api/2/myself") {
		t.Errorf("doctor = %v, logged %q, stderr %q; want the request in the file and on stderr", err, logged, printed.stderr)
	}
}

func TestVerboseWebSaysTheServersRequestsAsDebugLines(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeConfigFor(t, dir, workingJira(t))

	askingJira := func(string) cli.RunWeb {
		return func(ctx context.Context, _ config.Config, deps webserver.Deps, _ webserver.Info, _ io.Writer) error {
			_, _ = deps.Jira.Issue(ctx, "PROJ-1")

			return nil
		}
	}

	var ran rootRun

	// Act
	_, stderr, err := executeRoot(
		t, place{dir: dir, home: t.TempDir()}, ran.runInterface, askingJira, "--verbose", "--web",
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
