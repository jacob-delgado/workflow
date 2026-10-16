// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook exports them (githooks(5)), and from a linked worktree
// GIT_DIR is an absolute path into the shared repository: every git these tests
// start, theirs, lefthook's or a seam's, would inherit it and work on that
// repository instead of its temporary one — committing to it, and pushing to
// its origin. The names are git's own list for the purpose,
// gittest.LocalVariables. The tests then run with every such git reading
// the tests' own configuration.
func TestMain(m *testing.M) {
	// Running the tests anyway would be running them against that repository.
	err := gittest.ClearLocalVariables()
	if err != nil {
		panic(err)
	}

	os.Exit(withTheTestsGit(m))
}

// withTheTestsGit runs the tests m holds with every git the process starts
// reading the tests' own configuration (gittest.Configure), written into a
// directory it removes once they have run, and returns their exit code: a
// developer's global hooks or signing key would change what a test's git
// does, and maintenance git left running could write into a test's
// repository as it is removed.
func withTheTestsGit(m *testing.M) int {
	configured, err := os.MkdirTemp("", "workflow-test-git-*")
	if err != nil {
		panic(err)
	}

	err = gittest.Configure(configured, os.Setenv)
	if err != nil {
		panic(err)
	}

	code := m.Run()

	err = os.RemoveAll(configured)
	if err != nil {
		panic(err)
	}

	return code
}

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	// Arrange
	// A repository made, committed to and branched, a push to origin, and a
	// finish that pulls and deletes a branch: each would land in the
	// repository a hook names, were git's environment inherited.
	fixtures := []string{
		"TestTheGitSeamsReportAPushGitRefuses",
		"TestTheGitSeamsCommitWhatIsStagedAndLeaveNoMessageBehind",
		"TestTheGitSeamsFinishAMergedBranch",
	}

	isolateGit(t)

	named := t.TempDir()
	git(t, named, "init", "--quiet")

	before := gittest.FilesUnder(t, named)

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^("+strings.Join(fixtures, "|")+")$", "-test.v")

	child.Env = append(os.Environ(), gittest.HookEnvironment(named)...)

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

func TestTheGitASeamStartsLeavesNoMaintenanceRunning(t *testing.T) {
	t.Parallel()

	// Arrange
	// A seam starts git in the process's environment, as proc starts every
	// program, whether or not its test isolates anything more. Since git 2.47
	// a commit returns with maintenance it detached from itself still
	// running, which can be writing into the repository as the test's TempDir
	// is removed.
	repo := t.TempDir()
	git(t, repo, "init", "--quiet")

	trace := t.TempDir()
	commit := proc.Command{
		Name: "git", Dir: repo, Env: []string{"GIT_TRACE2_EVENT=" + trace},
		Args: []string{
			"-c", "user.email=t@example.com", "-c", "user.name=Tester", "commit", "--quiet", "--allow-empty", "-m", "work",
		},
	}

	// Act
	_, err := proc.RunCommand(t.Context(), commit)
	// Assert
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}

	if started := gittest.MaintenanceStarted(t, trace); len(started) != 0 || len(gittest.FilesUnder(t, trace)) == 0 {
		t.Errorf("the commit started %q, want it traced and no maintenance to outlive it", started)
	}
}
