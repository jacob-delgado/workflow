// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
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

// unnamedHost is a remote on a host that names no forge.
const unnamedHost = "git@git.example.com:a/b.git"

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
	isolateGit(t)

	root := repository(t)
	git(t, root, "remote", "add", "origin", githubRemote)

	inside := filepath.Join(root, "docs")

	err := os.Mkdir(inside, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	where := wiring.Locate(t.Context(), inside)
	if where.Root != root || where.Remote != githubRemote {
		t.Errorf("Locate = %+v, want root %s and origin", where, root)
	}

	outside := t.TempDir()
	if where := wiring.Locate(t.Context(), outside); where.Root != outside || where.Remote != "" {
		t.Errorf("Locate outside a repository = %+v, want the directory itself", where)
	}
}

func TestTheGitSeamsWorkOnARealRepository(t *testing.T) {
	isolateGit(t)

	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)

	root := repository(t)
	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: root, Remote: ""}).Git

	err := seams.CreateBranch(featureBranch, "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	branch, err := seams.Branch()
	if err != nil || branch.Name != featureBranch || branch.Base != "main" {
		t.Fatalf("Branch = %+v, %v", branch, err)
	}

	write(t, filepath.Join(root, "new.go"), "package x\n", 0o600)
	change := onlyChange(t, seams)

	stagedThenUnstaged(t, seams, change)

	err = seams.Stage(change)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	committed(t, seams, root, drafts)

	// With no origin a push fails, through git, with git's reason.
	push, err := seams.Push(featureBranch)
	if err != nil {
		t.Fatalf("Push did not start: %v", err)
	}

	_, err = drained(push)
	if err == nil {
		t.Error("a push with no origin succeeded")
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

// stagedThenUnstaged stages a change, checks it, and takes it back out.
func stagedThenUnstaged(t *testing.T, seams tui.GitDeps, change gitrepo.Change) {
	t.Helper()

	err := seams.Stage(change)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if staged := onlyChange(t, seams); !staged.IsStaged() {
		t.Errorf("after Stage, %+v is not staged", staged)
	}

	err = seams.Unstage(change)
	if err != nil {
		t.Fatalf("Unstage: %v", err)
	}

	if unstaged := onlyChange(t, seams); unstaged.IsStaged() {
		t.Errorf("after Unstage, %+v is still staged", unstaged)
	}
}

// committed commits the index, and checks the message file did not outlive the
// commit.
func committed(t *testing.T, seams tui.GitDeps, root, drafts string) {
	t.Helper()

	output, err := seams.Commit("feat: add x\n\nWhy.\n")
	if err != nil {
		t.Fatalf("Commit did not start: %v", err)
	}

	lines, err := drained(output)
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

func TestACommitThatCannotStartSaysWhy(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}).Git

	_, err := seams.Commit("feat: x\n")
	if err == nil || !strings.Contains(err.Error(), "writing the commit message") {
		t.Errorf("Commit = %v, want the message file's failure", err)
	}
}

func TestACommitGitCannotRunLeavesNoMessageBehind(t *testing.T) {
	drafts := t.TempDir()
	t.Setenv("TMPDIR", drafts)
	t.Setenv("PATH", t.TempDir())

	seams := wiring.Deps(t.Context(), config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}).Git

	_, err := seams.Commit("feat: x\n")
	if err == nil || !strings.Contains(err.Error(), "starting git commit") {
		t.Errorf("Commit = %v, want git's failure to start", err)
	}

	left, _ := filepath.Glob(filepath.Join(drafts, "*"))
	if len(left) != 0 {
		t.Errorf("the message file outlived a commit that never started: %q", left)
	}
}
