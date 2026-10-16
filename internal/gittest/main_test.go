// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gittest_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

// TestMain clears the variables that tell git which repository to use before
// any test runs. A hook exports them (githooks(5)), and from a linked worktree
// GIT_DIR is an absolute path into the shared repository: the gits these tests
// run to init, commit and push would inherit it and work on that repository
// instead of their temporary ones, committing to its checked-out branch.
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
	// The tests that commit and push through Run.
	fixtures := []string{
		"TestRunLeavesNoMaintenanceRunningAfterACommit", "TestRunLeavesNoMaintenanceRunningInTheRepositoryItPushesTo",
	}

	named := t.TempDir()
	gittest.Run(t, named, "init", "--quiet")

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
