// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook exports them (githooks(5)), and from a linked worktree
// GIT_DIR is an absolute path into the shared repository: every git these tests
// start, theirs, lefthook's or a seam's, would inherit it and work on that
// repository instead of its temporary one — committing to it, and pushing to
// its origin. The names are git's own list for the purpose,
// gittest.LocalVariables.
func TestMain(m *testing.M) {
	// Running the tests anyway would be running them against that repository.
	err := gittest.ClearLocalVariables()
	if err != nil {
		panic(err)
	}

	os.Exit(m.Run())
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
