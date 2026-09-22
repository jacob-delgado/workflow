// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// errPromptClosed stands in for a confirmation prompt that cannot be read.
var errPromptClosed = errors.New("prompt closed")

// issueFixture is the Jira Data Center body for one issue: its key, type and
// summary, which is all the branch name is composed from.
func issueFixture(key, issueType, summary string) string {
	return `{"key":"` + key + `","fields":{"summary":"` + summary + `",` +
		`"issuetype":{"name":"` + issueType + `"},` +
		`"status":{"name":"To Do","statusCategory":{"key":"new"}}}}`
}

// gitOutput runs one git command in dir and returns its output, keeping the
// developer's own git configuration out as run does.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}

	return string(out)
}

// currentBranch is the branch dir has checked out.
func currentBranch(t *testing.T, dir string) string {
	t.Helper()

	return strings.TrimSpace(gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
}

// repoForBranch is a repository with one commit — so a branch can be cut from
// HEAD — and a configuration pointing Jira at jiraURL.
func repoForBranch(t *testing.T, jiraURL string) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", "init")
	writeConfigFor(t, repo, jiraURL)

	return repo
}

func TestBranchCreatesTheBranchForTheIssue(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	output, err := run(t, repo, "branch", "PROJ-7", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("branch: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Created fix/PROJ-7-login") {
		t.Errorf("output does not report the branch was created:\n%s", output)
	}

	if got := currentBranch(t, repo); got != "fix/PROJ-7-login" {
		t.Errorf("checked-out branch = %q, want the created branch", got)
	}
}

func TestBranchRefusesAnIssueThatAlreadyHasABranch(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	git(t, repo, "branch", "fix/PROJ-7-login")

	// Act
	output, err := run(t, repo, "branch", "PROJ-7", "--yes")

	// Assert
	// The existing branch is switched to in the interface, never recreated here.
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("branch = %v, want a refusal naming the existing branch:\n%s", err, output)
	}
}

func TestBranchDryRunCreatesNothing(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Story", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	before := currentBranch(t, repo)

	// Act
	output, err := run(t, repo, "branch", "PROJ-7", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("branch --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "dry run: would create feat/PROJ-7-login") {
		t.Errorf("output does not describe the dry run:\n%s", output)
	}

	if got := currentBranch(t, repo); got != before {
		t.Errorf("a dry run left the repository on %q, want it unchanged from %q", got, before)
	}
}

func TestBranchCreatesAfterConfirmation(t *testing.T) {
	// Arrange
	// The confirmation is answered "yes", so the branch is created.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	output, err := runGuided(t, repo, scripted([]string{"y"}, nil), "branch", "PROJ-7")
	// Assert
	if err != nil {
		t.Fatalf("branch: %v (%s)", err, output)
	}

	if got := currentBranch(t, repo); got != "fix/PROJ-7-login" {
		t.Errorf("a confirmed branch was not created: on %q", got)
	}
}

func TestBranchDeclinedCreatesNothing(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	before := currentBranch(t, repo)

	// Act
	// The confirmation is answered "no".
	output, err := runGuided(t, repo, scripted([]string{"n"}, nil), "branch", "PROJ-7")
	// Assert
	if err != nil {
		t.Fatalf("branch: %v (%s)", err, output)
	}

	if !strings.Contains(output, "Not created.") {
		t.Errorf("output does not report the branch was left uncreated:\n%s", output)
	}

	if got := currentBranch(t, repo); got != before {
		t.Errorf("a declined branch left the repository on %q, want %q", got, before)
	}
}

func TestBranchCutsFromHeadWithoutAConventionalBase(t *testing.T) {
	// Arrange
	// The repository's only branch is neither main nor master, so there is no
	// base to name and the branch is cut from HEAD.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := t.TempDir()
	gitInit(t, repo)
	git(t, repo, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", "init")
	git(t, repo, "branch", "-M", "trunk")
	writeConfigFor(t, repo, server.URL)

	// Act
	output, err := run(t, repo, "branch", "PROJ-7", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("branch --dry-run: %v (%s)", err, output)
	}

	if !strings.Contains(output, "from HEAD") {
		t.Errorf("expected the branch to be cut from HEAD:\n%s", output)
	}
}

func TestBranchStopsWhenTheConfirmationCannotBeRead(t *testing.T) {
	// Arrange
	// The confirmation prompt fails, so the branch is not created.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	before := currentBranch(t, repo)
	failing := cli.Prompt{
		Line:   func(string) (string, error) { return "", errPromptClosed },
		Secret: func(string) (string, error) { return "", errPromptClosed },
	}

	// Act
	_, err := runGuided(t, repo, failing, "branch", "PROJ-7")

	// Assert
	if err == nil {
		t.Error("branch proceeded despite an unreadable confirmation")
	}

	if got := currentBranch(t, repo); got != before {
		t.Errorf("a failed confirmation created a branch: now on %q", got)
	}
}

func TestBranchReportsAFailedCreate(t *testing.T) {
	// Arrange
	// A branch named "fix" blocks creating "fix/PROJ-7-login": a ref cannot be
	// both a name and a directory of names.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	git(t, repo, "branch", "fix")

	// Act
	_, err := run(t, repo, "branch", "PROJ-7", "--yes")

	// Assert
	if err == nil {
		t.Error("branch reported no error though the create could not succeed")
	}
}

func TestBranchReportsAnUnreadableRepository(t *testing.T) {
	// Arrange
	// The tracker is reachable, but the working directory is not a repository, so
	// the existing branches cannot be listed.
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	dir := t.TempDir()
	writeConfigFor(t, dir, server.URL)

	// Act
	_, err := run(t, dir, "branch", "PROJ-7", "--yes")

	// Assert
	if err == nil {
		t.Error("branch reported no error outside a repository")
	}
}

func TestBranchReportsAnUnreachableTracker(t *testing.T) {
	// Arrange
	// The tracker answers every request with a server error, so the issue cannot
	// be read and no branch is named.
	server := jiraServer(t, http.StatusInternalServerError, "boom", new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	_, err := run(t, repo, "branch", "PROJ-7", "--yes")

	// Assert
	if err == nil {
		t.Error("branch accepted an issue it could not read")
	}
}
