// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package tui_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

func TestTheTestsLeaveTheRepositoryAHookNamesAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// The fixture that starts git: a repository made where setup writes the
	// file. Under a linked worktree's git directory, whose configuration is
	// the shared repository's, its git init would mark that repository bare.
	const fixture = "TestSetUpInARepositorySaysToIgnoreTheFile"

	hooked := gittest.LinkedWorktreeHook(t)
	before := gittest.FilesUnder(t, hooked.Repo)

	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+fixture+"$", "-test.v")

	child.Env = append(os.Environ(), hooked.Environment...)

	// Act
	output, err := child.CombinedOutput()

	// Assert
	if changed := gittest.ChangedFiles(before, gittest.FilesUnder(t, hooked.Repo)); len(changed) != 0 {
		t.Errorf("the tests wrote into the repository git's environment names: %q", changed)
	}

	if err != nil {
		t.Fatalf("the tests failed with git's environment naming another repository: %v\n%s", err, output)
	}

	if !strings.Contains(string(output), "--- PASS: "+fixture+" ") {
		t.Errorf("%s did not pass with git's environment naming another repository:\n%s", fixture, output)
	}
}
