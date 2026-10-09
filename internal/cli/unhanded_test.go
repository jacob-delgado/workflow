// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli"
)

// unhanded is what a command says when it was run without the environment it
// runs in.
const unhanded = "without the environment it runs in"

// configShow is a command that reads the working directory first.
func configShow() []string {
	return strings.Fields("config show")
}

// outcome is how a run of the command tree ended: the error it returned, or
// what it panicked with.
type outcome struct {
	err      error
	panicked any
}

// executeCatching runs root, telling a panic apart from an error.
func executeCatching(t *testing.T, root *cobra.Command) outcome {
	t.Helper()

	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	ended := make(chan outcome, 1)

	go func() {
		defer func() {
			if caught := recover(); caught != nil {
				ended <- outcome{err: nil, panicked: caught}
			}
		}()

		ended <- outcome{err: root.ExecuteContext(t.Context()), panicked: nil}
	}()

	return <-ended
}

// NewRootCmd builds a tree to walk, as the reference generator does: a command
// run from it anyway says what it lacks, rather than reading what it was never
// handed.
func TestACommandRunFromATreeBuiltToWalkSaysItHasNoEnvironment(t *testing.T) {
	t.Parallel()

	// Arrange
	root := cli.NewRootCmd(cli.Prompt{})
	root.SetArgs(configShow())

	// Act
	ended := executeCatching(t, root)

	// Assert
	if ended.panicked != nil || ended.err == nil || !strings.Contains(ended.err.Error(), unhanded) {
		t.Errorf("config show from a tree built to walk = %+v, want it to say it has no environment", ended)
	}
}

// A subcommand with a persistent hook of its own keeps the root's from
// running, so nothing hands it the environment: it says so rather than
// reading what it was never handed.
func TestACommandWhoseOwnHookKeepsTheRootsSaysItHasNoEnvironment(t *testing.T) {
	t.Parallel()

	// Arrange
	root := cli.NewRootCmdOver(unusedPrompt(t), nil, nil, environmentFor(t, place{dir: t.TempDir(), home: t.TempDir()}))
	root.SetArgs(configShow())

	show, _, err := root.Find(configShow())
	if err != nil {
		t.Fatalf("finding config show: %v", err)
	}

	show.PersistentPreRun = func(*cobra.Command, []string) {}

	// Act
	ended := executeCatching(t, root)

	// Assert
	if ended.panicked != nil || ended.err == nil || !strings.Contains(ended.err.Error(), unhanded) {
		t.Errorf("config show under a hook of its own = %+v, want it to say it has no environment", ended)
	}
}
