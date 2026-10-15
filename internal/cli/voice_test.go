// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"io"
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
