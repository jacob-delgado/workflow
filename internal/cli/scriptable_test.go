// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// The scriptable write commands are wired end to end here; without a tracker,
// a repository or Slack they fail with a reason rather than a panic.

func TestBranchCommandNeedsATracker(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "branch", "PROJ-1")

	// Assert
	if err == nil {
		t.Error("branch for an issue with no tracker configured returned no error")
	}
}

func TestPRCommandReadsTheBranch(t *testing.T) {
	// Act
	// Outside a repository there is no branch to open a pull request for.
	_, err := run(t, t.TempDir(), "pr")

	// Assert
	if err == nil {
		t.Error("pr outside a repository returned no error")
	}
}

func TestAnnounceCommandNeedsMessaging(t *testing.T) {
	// Act
	_, err := run(t, t.TempDir(), "announce")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "messaging") {
		t.Errorf("announce with no messaging returned %v, want a not-configured error", err)
	}

	wantExit(t, err, 3)
}

func TestDeclineNoticeGoesToStderr(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, scripted([]string{"n"}, nil), "branch", "PROJ-7")
	if err != nil {
		t.Fatalf("branch: %v (%+v)", err, printed)
	}

	// Assert
	// The preview is the artifact a script reads; the decline is said about it.
	if !strings.Contains(printed.stdout, "Branch fix/PROJ-7-login") ||
		!strings.Contains(printed.stderr, "Not created.") || strings.Contains(printed.stdout, "Not created.") {
		t.Errorf("the preview or the decline is on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestDryRunLineGoesToStderr(t *testing.T) {
	// Arrange
	repo := prRepo(t, "fix/PROJ-2-thing")

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "pr", "--dry-run")
	if err != nil {
		t.Fatalf("pr --dry-run: %v (%+v)", err, printed)
	}

	// Assert
	if !strings.HasPrefix(printed.stdout, "Open ") || !strings.Contains(printed.stdout, "fix/PROJ-2-thing → main") ||
		!strings.Contains(printed.stderr, "dry run: would") || strings.Contains(printed.stdout, "dry run:") {
		t.Errorf("the preview or the dry-run line is on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

func TestConfirmWithoutATerminalNamesYes(t *testing.T) {
	// Arrange
	// Stdin is piped and empty, so the confirmation reads end-of-file.
	repo := prRepo(t, "fix/PROJ-2-thing")
	closed := cli.Prompt{Line: func(string) (string, error) { return "", io.EOF }}

	// Act
	_, err := runGuided(t, repo, closed, "pr")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "pass --yes") {
		t.Errorf("pr with no terminal = %v, want it to say to pass --yes", err)
	}

	wantExit(t, err, 2)
}

func TestConfigInitSaysWhatItWroteOnStderr(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	printed, err := runStreams(t, dir, unusedPrompt(t), "config", "init", "--template")
	if err != nil {
		t.Fatalf("config init --template: %v (%+v)", err, printed)
	}

	// Assert
	// The artifact of config init is the file; what it says about the file,
	// and what to do next, is commentary.
	if printed.stdout != "" || !strings.Contains(printed.stderr, "Wrote ") || !strings.Contains(printed.stderr, "Next:") {
		t.Errorf("config init put its notices on the wrong stream:\nstdout:\n%s\nstderr:\n%s",
			printed.stdout, printed.stderr)
	}
}

// searchFixture is the Jira body for a search of the issues assigned to you.
func searchFixture(keys ...string) string {
	issues := make([]string, 0, len(keys))
	for _, key := range keys {
		issues = append(issues, `{"key":"`+key+`","fields":{"summary":"x",`+
			`"issuetype":{"name":"Bug"},"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
	}

	return `{"total":` + strconv.Itoa(len(keys)) + `,"issues":[` + strings.Join(issues, ",") + `]}`
}

// completeBranch runs the shell-completion hook for `workflow branch <toComplete>`
// in dir, the way a shell would, and returns everything it offered.
func completeBranch(t *testing.T, dir, toComplete string) string {
	t.Helper()

	output, err := run(t, dir, "__complete", "branch", toComplete)
	if err != nil {
		t.Fatalf("__complete branch %q: %v (%s)", toComplete, err, output)
	}

	return output
}

func TestBranchCompletionOffersAssignedIssues(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, searchFixture("PROJ-7", "PROJ-8"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	if !strings.Contains(output, "PROJ-7") || !strings.Contains(output, "PROJ-8") {
		t.Errorf("completion does not offer the assigned issues:\n%s", output)
	}
}

func TestBranchCompletionFiltersByPrefix(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, searchFixture("PROJ-7", "TASK-3"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "PROJ")

	// Assert
	if !strings.Contains(output, "PROJ-7") {
		t.Errorf("completion drops a key that matches the typed prefix:\n%s", output)
	}

	if strings.Contains(output, "TASK-3") {
		t.Errorf("completion offers a key that does not match the typed prefix:\n%s", output)
	}
}

func TestBranchCompletionOffersNothingWhenTheTrackerCannotBeReached(t *testing.T) {
	// Arrange
	// The tracker would have keys to offer, but every request fails.
	server := jiraServer(t, http.StatusInternalServerError, searchFixture("PROJ-7"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	output := completeBranch(t, dir, "")

	// Assert
	// Completion must not spill an error into the shell: the key the tracker
	// would have offered is absent, not surfaced as a failure.
	if strings.Contains(output, "PROJ-7") {
		t.Errorf("completion offered a key from a tracker it could not read:\n%s", output)
	}
}
