// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook or a rebase's exec exports them, and from a linked
// worktree GIT_DIR names that worktree's git directory, whose configuration
// is the shared repository's: the git init a test starts would take it for
// the repository to make and mark the shared one bare. The names are git's
// own list for the purpose, gittest.LocalVariables.
func TestMain(m *testing.M) {
	// Running the tests anyway would be running them against that repository.
	err := gittest.ClearLocalVariables()
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// The fixture that starts git: a repository made where the file is written.
	// A hook or a rebase's exec in a linked worktree names that worktree's git
	// directory, whose configuration is the shared repository's: a git init
	// there marks the shared repository bare.
	fixtures := []string{"TestWriteNamesAFileGitWouldCommit"}

	named := t.TempDir()
	gittest.Run(t, named, "init", "--quiet")
	gittest.Run(t, named, "-c", "user.name=Tester", "-c", "user.email=t@example.com",
		"commit", "--quiet", "--allow-empty", "-m", "first")
	gittest.Run(t, named, "worktree", "add", "--quiet", "--detach", t.TempDir())

	before := gittest.FilesUnder(t, named)

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^("+strings.Join(fixtures, "|")+")$", "-test.v")

	child.Env = append(os.Environ(), "GIT_DIR="+linkedGitDir(t, named))

	// Act
	output, err := child.CombinedOutput()

	// Assert
	if changed := gittest.ChangedFiles(before, gittest.FilesUnder(t, named)); len(changed) != 0 {
		t.Errorf("the tests wrote into the repository git's environment names: %q", changed)
	}

	if err != nil {
		t.Fatalf("the tests failed with git's environment naming another repository: %v\n%s", err, output)
	}

	for _, fixture := range fixtures {
		if !strings.Contains(string(output), "--- PASS: "+fixture+" ") {
			t.Errorf("%s did not pass with git's environment naming another repository:\n%s", fixture, output)
		}
	}
}

// linkedGitDir is the git directory of the one linked worktree of repo.
func linkedGitDir(t *testing.T, repo string) string {
	t.Helper()

	worktrees, err := os.ReadDir(filepath.Join(repo, ".git", "worktrees"))
	if err != nil || len(worktrees) != 1 {
		t.Fatalf("the linked worktree of %s: %v, %d found", repo, err, len(worktrees))
	}

	return filepath.Join(repo, ".git", "worktrees", worktrees[0].Name())
}
