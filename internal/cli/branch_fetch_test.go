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

// loginBranch is the branch PROJ-7, a bug titled "login", is named.
const loginBranch = "fix/PROJ-7-login"

// repoWithOrigin is repoForBranch with a bare origin whose main the
// repository has fetched, so a new branch starts from origin/main.
func repoWithOrigin(t *testing.T, jiraURL string) (string, string) {
	t.Helper()

	repo := repoForBranch(t, jiraURL)
	origin := t.TempDir()
	git(t, origin, "init", "--bare", "--quiet")
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "--quiet", "origin", "HEAD:refs/heads/main")
	git(t, repo, "fetch", "--quiet", "origin")
	git(t, repo, "remote", "set-head", "origin", "main")

	return repo, origin
}

// advanceOrigin commits once more on origin's main from another clone, which
// the repository has not fetched, and returns that commit.
func advanceOrigin(t *testing.T, origin string) string {
	t.Helper()

	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "--quiet", "--branch", "main", origin, other)
	git(t, other, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", "since")
	git(t, other, "push", "--quiet", "origin", "HEAD:main")

	return strings.TrimSpace(gitOutput(t, other, "rev-parse", "HEAD"))
}

func TestBranchFetchStartsFromWhatOriginHoldsNow(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo, origin := repoWithOrigin(t, server.URL)
	latest := advanceOrigin(t, origin)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "branch", "PROJ-7", "--fetch", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("branch --fetch: %v (%+v)", err, printed)
	}

	// Only a fetch before the create can start the branch at a commit the
	// repository had never seen.
	if got := strings.TrimSpace(gitOutput(t, repo, "rev-parse", loginBranch)); got != latest {
		t.Errorf("fix/PROJ-7-login starts at %s, want origin's newest commit %s", got, latest)
	}
}

func TestBranchFetchSaysItFetchesFirst(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo, _ := repoWithOrigin(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "branch", "PROJ-7", "--fetch", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("branch --fetch --dry-run: %v (%+v)", err, printed)
	}

	const plan = "fetch origin, then create fix/PROJ-7-login from origin/main and switch to it"
	if !strings.Contains(printed.stdout, "Start work on PROJ-7: "+plan) ||
		!strings.Contains(printed.stderr, "dry run: would start work on PROJ-7: "+plan) {
		t.Errorf("branch --fetch does not say it fetches first:\nstdout:\n%s\nstderr:\n%s", printed.stdout, printed.stderr)
	}
}

func TestBranchFetchThatFailsCreatesNothing(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo, origin := repoWithOrigin(t, server.URL)
	before := currentBranch(t, repo)

	err := os.RemoveAll(origin)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = run(t, repo, "branch", "PROJ-7", "--fetch", "--yes")

	// Assert
	// The way out is the terminal's "branch from what you have".
	if err == nil || !strings.Contains(err.Error(), "fetching origin") ||
		!strings.Contains(err.Error(), "without --fetch") {
		t.Errorf("branch --fetch with origin gone = %v, want the fetch named and the way to branch without it", err)
	}

	if got := currentBranch(t, repo); got != before {
		t.Errorf("a failed fetch left the repository on %q, want %q", got, before)
	}
}

func TestBranchWorktreePrintsTheDirectoryItMade(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)
	before := currentBranch(t, repo)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "branch", "PROJ-7", "--worktree", "--yes")
	// Assert
	if err != nil {
		t.Fatalf("branch --worktree: %v (%+v)", err, printed)
	}

	lines := strings.Split(strings.TrimSpace(printed.stdout), "\n")
	dir := lines[len(lines)-1]

	if got := currentBranch(t, dir); got != loginBranch {
		t.Errorf("the last line of stdout, %q, is no worktree on fix/PROJ-7-login (on %q)", dir, got)
	}

	if filepath.Dir(dir) != filepath.Dir(repo) {
		t.Errorf("the worktree %s is not beside the repository %s", dir, repo)
	}

	if got := currentBranch(t, repo); got != before {
		t.Errorf("a worktree moved the repository's own checkout to %q, want it left on %q", got, before)
	}
}

func TestBranchWorktreeDryRunSaysWhereItWouldWork(t *testing.T) {
	// Arrange
	server := jiraServer(t, http.StatusOK, issueFixture("PROJ-7", "Bug", "login"), new(atomic.Bool))
	repo := repoForBranch(t, server.URL)

	// Act
	printed, err := runStreams(t, repo, unusedPrompt(t), "branch", "PROJ-7", "--worktree", "--dry-run")
	// Assert
	if err != nil {
		t.Fatalf("branch --worktree --dry-run: %v (%+v)", err, printed)
	}

	if !strings.Contains(printed.stderr, "dry run: would start work on PROJ-7: create fix/PROJ-7-login from ") ||
		!strings.HasSuffix(strings.TrimSpace(printed.stderr), " in a new worktree beside the repository") {
		t.Errorf("branch --worktree --dry-run does not say it makes a worktree:\n%s", printed.stderr)
	}

	_, err = os.Stat(repo + "-fix-PROJ-7-login")
	if err == nil {
		t.Error("a dry run made the worktree")
	}
}
