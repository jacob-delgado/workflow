// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// The scriptable write commands are wired end to end here; without a tracker,
// a repository or Slack they fail with a reason rather than a panic.

func TestBranchCommandNeedsATracker(t *testing.T) {
	t.Parallel()

	// Act
	// With no Jira configured the forge's issues are the tracker, and with no
	// repository there is no remote to name the forge, so there is no tracker to
	// read issue 1 from.
	_, err := clitest.Run(t, t.TempDir(), "branch", "1")

	// Assert
	if !errors.Is(err, forge.ErrNotARemote) {
		t.Errorf("branch with no tracker to read from returned %v, want ErrNotARemote", err)
	}

	wantExit(t, err, 3)
}

func TestPRCommandReadsTheBranch(t *testing.T) {
	t.Parallel()

	// Act
	// Outside a repository there is no branch to open a pull request for.
	_, err := clitest.Run(t, t.TempDir(), "pr")

	// Assert
	if !errors.Is(err, gitrepo.ErrNotARepository) {
		t.Errorf("pr outside a repository returned %v, want ErrNotARepository", err)
	}

	wantExit(t, err, 4)
}

func TestAnnounceCommandNeedsMessaging(t *testing.T) {
	t.Parallel()

	// Act
	_, err := clitest.Run(t, t.TempDir(), "announce")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "messaging") {
		t.Errorf("announce with no messaging returned %v, want a not-configured error", err)
	}

	wantExit(t, err, 3)
}

func TestDeclineNoticeGoesToStderr(t *testing.T) {
	t.Parallel()

	// Arrange
	server := clitest.JiraServer(t, http.StatusOK, clitest.IssueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.Scripted([]string{"n"}, nil), "branch", "PROJ-7")
	if err != nil {
		t.Fatalf("branch: %v (%+v)", err, printed)
	}

	// Assert
	// The preview is the artifact a script reads; the decline is said about it.
	if !strings.Contains(printed.Stdout, "Start work on PROJ-7: create fix/PROJ-7-login") ||
		!strings.Contains(printed.Stderr, "Not created.") || strings.Contains(printed.Stdout, "Not created.") {
		t.Errorf("the preview or the decline is on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}
}

func TestDryRunLineGoesToStderr(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	printed, err := clitest.RunStreams(t, repo, clitest.UnusedPrompt(t), "pr", "--dry-run")
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.HasPrefix(printed.Stdout, "Open ") || !strings.Contains(printed.Stdout, "fix/PROJ-2-thing → main") ||
		!strings.Contains(printed.Stderr, "dry run: would") || strings.Contains(printed.Stdout, "dry run:") {
		t.Errorf("the preview or the dry-run line is on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}
}

func TestConfirmWithoutATerminalNamesYes(t *testing.T) {
	t.Parallel()

	// Arrange
	// Stdin is piped and empty, so the confirmation reads end-of-file.
	repo := prRepo(t, "fix/PROJ-2-thing")
	closed := cmdkit.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := clitest.RunGuided(t, repo, closed, "pr")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Errorf("pr with no terminal = %v, want it to say to pass --yes", err)
	}

	wantExit(t, err, 2)
}

func TestConfigInitSaysWhatItWroteOnStderr(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "config", "init", "--template")
	if err != nil {
		t.Fatalf("config init --template: %v (%+v)", err, printed)
	}

	// Assert
	// The artifact of config init is the file; what it says about the file,
	// and what to do next, is commentary.
	if printed.Stdout != "" || !strings.Contains(printed.Stderr, "Wrote ") || !strings.Contains(printed.Stderr, "Next:") {
		t.Errorf("config init put its notices on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.Stdout, printed.Stderr)
	}
}
