// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package issues_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/gittest"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook exports them (githooks(5)), and from a linked worktree
// GIT_DIR is an absolute path into the shared repository: every git these
// tests start, theirs or the command's under test, would inherit it and work
// on that repository instead of its temporary one. The names are git's own
// list for the purpose, gittest.LocalVariables. The tests then run in the
// process clitest.Main readies for them.
func TestMain(m *testing.M) {
	// Running the tests anyway would be running them against that repository.
	err := gittest.ClearLocalVariables()
	if err != nil {
		panic(err)
	}

	os.Exit(clitest.Main(m))
}

// wantExit fails the test unless err exits with want. It lives beside the
// tests that assert with it, since cmd/testshape follows an Assert's failure
// helper only within its own test package.
func wantExit(t *testing.T, err error, want int) {
	t.Helper()

	if got := cli.ExitStatus(err); got != want {
		t.Errorf("ExitStatus(%v) = %d, want %d", err, got, want)
	}
}

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository made, committed to and branched, then its forge found from
	// its origin: with git's environment inherited, each of those would land
	// in the repository a hook names instead.
	const fixture = "TestIssuesListsTheForgesIssuesBesideTheFirstView"

	named := t.TempDir()
	gittest.Run(t, named, "init", "--quiet")

	before := gittest.FilesUnder(t, named)

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+fixture+"$", "-test.v")

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

	if !strings.Contains(string(output), "--- PASS: "+fixture+" ") {
		t.Errorf("%s did not pass with git's environment naming another repository:\n%s", fixture, output)
	}
}
