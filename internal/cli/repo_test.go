// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// The repositories the tests run commands in, made with git itself. They live
// in a _test.go file, not internal/cli/clitest, since only a test may run git
// (.golangci.yml's gittest-only-in-tests).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/gittest"
)

// githubRepo is a repository on branch, one commit ahead of main, with a GitHub
// origin so the forge resolves to gh's dialect.
func githubRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := prRepo(t, branch)
	git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")

	return repo
}

// pretendPushed gives the checked-out branch a local upstream on origin with
// nothing ahead, so the pull-request flow treats it as already published and
// skips the push — there is no real remote to push to in a black-box test.
func pretendPushed(t *testing.T, repo string) {
	t.Helper()

	branch := currentBranch(t, repo)
	git(t, repo, "update-ref", "refs/remotes/origin/"+branch, "HEAD")
	git(t, repo, "branch", "--set-upstream-to=origin/"+branch, branch)
}

// gitInit makes dir a real repository, so doctor's repository section can be
// tested against git itself rather than against an imitation of it.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "--quiet", ".")
}

// git runs one git command in dir, failing the test if it does not succeed. The
// developer's own git configuration is kept out, as clitest.Run keeps it out
// of doctor.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	gittest.Run(t, dir, args...)
}

// gitOutput runs one git command in dir and returns its output, keeping the
// developer's own git configuration out as clitest.Run does.
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

// commit makes an empty commit in dir with a fixed identity.
func commit(t *testing.T, dir, message string) {
	t.Helper()

	git(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", message)
}

// prRepo is a repository on branch, one commit ahead of main — a branch a pull
// request can be opened for.
func prRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", branch)
	commit(t, repo, "work")

	return repo
}

// ownedRepo is a GitHub repository whose main holds a CODEOWNERS giving the
// API to ana, a team and octo — the fake gh's own user — on a branch that
// changes the API and is already pushed.
func ownedRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitInit(t, repo)
	writeRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @ana @acme/control-plane @octo\n")
	git(t, repo, "add", ".")
	commit(t, repo, "init")
	git(t, repo, "branch", "-M", "main")
	git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	writeRepoFile(t, repo, "api/pull.go", "package api\n")
	git(t, repo, "add", ".")
	commit(t, repo, "fix: guard the api")
	git(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	pretendPushed(t, repo)
	clitest.WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

// writeRepoFile writes contents to a path under repo, creating parent directories
// — for a repository file the command reads, such as a pull request template.
func writeRepoFile(t *testing.T, repo, rel, contents string) {
	t.Helper()

	path := filepath.Join(repo, rel)

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}

	clitest.WriteExecutable(t, path, contents, 0o644)
}
