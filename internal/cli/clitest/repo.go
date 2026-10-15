// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

// GithubRepo is a repository on branch, one commit ahead of main, with a GitHub
// origin so the forge resolves to gh's dialect.
func GithubRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := PRRepo(t, branch)
	Git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")

	return repo
}

// GitlabRepo is PRRepo on branch with a GitLab origin and gitlabForgeConfig.
func GitlabRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := PRRepo(t, branch)
	Git(t, repo, "remote", "add", "origin", "https://gitlab.com/owner/repo.git")
	WriteFile(t, repo, gitlabForgeConfig)

	return repo
}

// PretendPushed gives the checked-out branch a local upstream on origin with
// nothing ahead, so the pull-request flow treats it as already published and
// skips the push — there is no real remote to push to in a black-box test.
func PretendPushed(t *testing.T, repo string) {
	t.Helper()

	branch := CurrentBranch(t, repo)
	Git(t, repo, "update-ref", "refs/remotes/origin/"+branch, "HEAD")
	Git(t, repo, "branch", "--set-upstream-to=origin/"+branch, branch)
}

// GitInit makes dir a real repository, so a command can be tested against git
// itself rather than against an imitation of it.
func GitInit(t *testing.T, dir string) {
	t.Helper()
	Git(t, dir, "init", "--quiet", ".")
}

// Git runs one git command in dir, failing the test if it does not succeed. The
// developer's own git configuration is kept out, as Run keeps it out of the
// command.
func Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	gittest.Run(t, dir, args...)
}

// RepoWithRemote makes a repository whose origin is remote.
func RepoWithRemote(t *testing.T, remote string) string {
	t.Helper()

	dir := t.TempDir()
	GitInit(t, dir)
	Git(t, dir, "remote", "add", "origin", remote)

	return dir
}

// GitOutput runs one git command in dir and returns what it wrote to standard
// output, trimmed, keeping the developer's own git configuration out as Run
// does.
func GitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	return gittest.Run(t, dir, args...)
}

// CurrentBranch is the branch dir has checked out.
func CurrentBranch(t *testing.T, dir string) string {
	t.Helper()

	return GitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// Commit makes an empty commit in dir with a fixed identity.
func Commit(t *testing.T, dir, message string) {
	t.Helper()

	Git(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=Tester",
		"commit", "--allow-empty", "-m", message)
}

// PRRepo is a repository on branch, one commit ahead of main — a branch a pull
// request can be opened for.
func PRRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := t.TempDir()
	GitInit(t, repo)
	Commit(t, repo, "init")
	Git(t, repo, "branch", "-M", "main")
	Git(t, repo, "checkout", "-b", branch)
	Commit(t, repo, "work")

	return repo
}

// OwnedRepo is a GitHub repository whose main holds a CODEOWNERS giving the
// API to ana, a team and octo — the fake gh's own user — on a branch that
// changes the API and is already pushed.
func OwnedRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	GitInit(t, repo)
	WriteRepoFile(t, repo, ".github/CODEOWNERS", "/api/ @ana @acme/control-plane @octo\n")
	Git(t, repo, "add", ".")
	Commit(t, repo, "init")
	Git(t, repo, "branch", "-M", "main")
	Git(t, repo, "checkout", "-b", "fix/PROJ-2-thing")
	WriteRepoFile(t, repo, "api/pull.go", "package api\n")
	Git(t, repo, "add", ".")
	Commit(t, repo, "fix: guard the api")
	Git(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	PretendPushed(t, repo)
	WriteFile(t, repo, `{"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}

// WriteRepoFile writes contents to a path under repo, creating parent
// directories — for a repository file the command reads, such as a pull
// request template.
func WriteRepoFile(t *testing.T, repo, rel, contents string) {
	t.Helper()

	path := filepath.Join(repo, rel)

	err := os.MkdirAll(filepath.Dir(path), dirMode)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}

	writeData(t, path, contents)
}

// FeatureRepo is PRRepo on PROJ-2's branch, fix/PROJ-2-thing: one commit on
// main and one on the branch.
func FeatureRepo(t *testing.T) string {
	t.Helper()

	return PRRepo(t, "fix/PROJ-2-thing")
}

// JiraFeatureRepo is a repository on a feature branch for PROJ-7, one commit
// ahead of main, with a GitHub remote so the forge resolves through the fake gh,
// and a configuration naming the Jira at jiraURL.
func JiraFeatureRepo(t *testing.T, jiraURL string) string {
	t.Helper()

	repo := GithubRepo(t, "fix/PROJ-7-login")
	WriteFile(t, repo, `{"jira":{"base_url":"`+jiraURL+`","token":"t"},`+
		`"forge":{"cli":true,"kind":"github","host":"github.com"}}`)

	return repo
}
