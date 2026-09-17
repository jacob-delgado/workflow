// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// githubRemote is a remote on github.com that no test ever reaches.
const githubRemote = "git@github.com:example/repo.git"

// unnamedHostName is a host that names no forge, and unnamedHost a remote on
// it.
const (
	unnamedHostName = "git.example.com"
	unnamedHost     = "git@" + unnamedHostName + ":a/b.git"
)

// githubKind is forge.kind naming GitHub.
const githubKind = "github"

// featureBranch is the branch the git tests start.
const featureBranch = "feat/PROJ-1-x"

// isolateGit keeps a developer's own git configuration — global hooks, a
// signing key, a default branch — out of a test.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// git runs git in dir, failing the test if it fails.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()

	output, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, output)
	}

	return strings.TrimSpace(string(output))
}

// repository is a new repository on main with one commit.
func repository(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	git(t, dir, "init", "--quiet", "--initial-branch=main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test")
	write(t, filepath.Join(dir, "README.md"), "hello\n", 0o600)
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "--quiet", "-m", "chore: start")

	return dir
}

// write writes a file, failing the test if it cannot.
func write(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), mode)
	if err != nil {
		t.Fatal(err)
	}
}

// drained reads every line of a program's output and reports how it ended.
func drained(output proc.Output) ([]string, error) {
	var lines []string

	for line := range output.Lines {
		lines = append(lines, line)
	}

	return lines, output.Wait()
}

func TestLocateFindsTheRepositoryFromAnywhereInIt(t *testing.T) {
	// Arrange
	isolateGit(t)

	root := repository(t)
	git(t, root, "remote", "add", "origin", githubRemote)

	inside := filepath.Join(root, "docs")

	err := os.Mkdir(inside, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	where := wiring.Locate(t.Context(), inside)

	// Assert
	if where.Root != root || where.Remote != githubRemote {
		t.Errorf("Locate = %+v, want root %s and origin", where, root)
	}
}

func TestLocateOutsideARepositoryIsTheDirectoryItself(t *testing.T) {
	// Arrange
	isolateGit(t)

	outside := t.TempDir()

	// Act
	where := wiring.Locate(t.Context(), outside)

	// Assert
	if where.Root != outside || where.Remote != "" {
		t.Errorf("Locate outside a repository = %+v, want the directory itself", where)
	}
}

// gitSeams are the git seams on a new repository, with git's own configuration
// kept out and drafts written under drafts.
func gitSeams(t *testing.T) (tui.GitDeps, string, string) {
	t.Helper()

	isolateGit(t)

	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)

	root := repository(t)

	return wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: root, Remote: ""}, nil).Git, root, drafts
}

func TestTheGitSeamsStartABranchFromMain(t *testing.T) {
	// Arrange
	seams, _, _ := gitSeams(t)

	// Act
	err := seams.CreateBranch(featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	// Assert
	branch, err := seams.Branch()
	if err != nil || branch.Name != featureBranch || branch.Base != "main" {
		t.Errorf("Branch = %+v, %v; want %s checked out, from main", branch, err, featureBranch)
	}
}

func TestTheGitSeamsStageAndUnstageAChange(t *testing.T) {
	// Arrange
	seams, root, _ := gitSeams(t)
	write(t, filepath.Join(root, "new.go"), "package x\n", 0o600)

	change := onlyChange(t, seams)

	// Act: stage it
	err := seams.Stage(change)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Assert: it is staged
	if staged := onlyChange(t, seams); !staged.IsStaged() {
		t.Errorf("after Stage, %+v is not staged", staged)
	}

	// Act: unstage it
	err = seams.Unstage(change)
	if err != nil {
		t.Fatalf("Unstage: %v", err)
	}

	// Assert: it is no longer staged
	if unstaged := onlyChange(t, seams); unstaged.IsStaged() {
		t.Errorf("after Unstage, %+v is still staged", unstaged)
	}
}

func TestTheGitSeamsCommitWhatIsStagedAndLeaveNoMessageBehind(t *testing.T) {
	// Arrange
	seams, root, drafts := gitSeams(t)
	write(t, filepath.Join(root, "new.go"), "package x\n", 0o600)

	err := seams.Stage(onlyChange(t, seams))
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	// Act
	output, err := seams.Commit("feat: add x\n\nWhy.\n")
	if err != nil {
		t.Fatalf("Commit did not start: %v", err)
	}

	lines, err := drained(output)
	// Assert
	if err != nil {
		t.Fatalf("Commit: %v (%q)", err, lines)
	}

	if subject := git(t, root, "log", "-1", "--format=%s"); subject != "feat: add x" {
		t.Errorf("committed %q, want the message given", subject)
	}

	left, _ := filepath.Glob(filepath.Join(drafts, "workflow-commit-*"))
	if len(left) != 0 {
		t.Errorf("the message file outlived the commit: %q", left)
	}
}

func TestTheGitSeamsReportAPushGitRefuses(t *testing.T) {
	// Arrange
	seams, _, _ := gitSeams(t)

	err := seams.CreateBranch(featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	// Act
	push, err := seams.Push(featureBranch)
	if err != nil {
		t.Fatalf("Push did not start: %v", err)
	}

	lines, err := drained(push)

	// Assert
	// With no origin a push fails, through git, with git's reason.
	if err == nil || !strings.Contains(strings.Join(lines, "\n"), "origin") {
		t.Errorf("Push ended %v saying %q, want git's refusal naming origin", err, lines)
	}
}

// onlyChange is the one change in the work tree.
func onlyChange(t *testing.T, seams tui.GitDeps) gitrepo.Change {
	t.Helper()

	changes, err := seams.Changes()
	if err != nil || len(changes) != 1 {
		t.Fatalf("Changes = %+v, %v; want one", changes, err)
	}

	return changes[0]
}

func TestACommitThatCannotWriteItsMessageSaysWhy(t *testing.T) {
	// Arrange
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Git

	// Act
	_, err := seams.Commit("feat: x\n")

	// Assert
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "writing the commit message") {
		t.Errorf("Commit = %v, want the message file's failure", err)
	}
}

func TestACommitGitCannotRunLeavesNoMessageBehind(t *testing.T) {
	// Arrange
	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)
	t.Setenv("PATH", t.TempDir())

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil).Git

	// Act
	_, err := seams.Commit("feat: x\n")

	// Assert
	if !errors.Is(err, proc.ErrNotFound) || !strings.Contains(err.Error(), "starting git commit") {
		t.Errorf("Commit = %v, want git's failure to start", err)
	}

	left, _ := filepath.Glob(filepath.Join(drafts, "*"))
	if len(left) != 0 {
		t.Errorf("the message file outlived a commit that never started: %q", left)
	}
}

func TestTheForgeSeamRetriesAfterAFailedConnection(t *testing.T) {
	// Arrange
	isolateGit(t)

	dir := t.TempDir()
	probe := filepath.Join(dir, "attempts")
	write(t, filepath.Join(dir, "gh"), "#!/bin/sh\nprintf 'x\\n' >> '"+probe+"'\nexit 1\n", 0o700)
	t.Setenv("PATH", dir)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	forgeSeam := wiring.Deps(t.Context(),
		config.Config{Forge: config.Forge{Kind: githubKind}},
		wiring.Workspace{Root: dir, Remote: githubRemote}, nil).Forge

	// Act
	_, _, first := forgeSeam.FindPullRequest(featureBranch)
	_, _, second := forgeSeam.FindPullRequest(featureBranch)

	// Assert
	if first == nil || second == nil {
		t.Fatalf("both lookups should fail without a token: %v, %v", first, second)
	}

	attempts, err := os.ReadFile(probe)
	if err != nil || strings.Count(string(attempts), "x") != 2 {
		t.Errorf("the forge was reached %q, want a second attempt rather than a cached failure", attempts)
	}
}
