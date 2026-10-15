// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

// The kit is what every command of workflow's tree is built from, so most of
// its tests build a small tree of their own over it, as the root builds
// workflow's, and run a probe command that calls the part of the kit under
// test.

// The command a tree here runs, and the root's flags its tests give it.
const (
	probeCommand = "probe"
	webFlag      = "--web"
	portFlag     = "--port"
	verboseFlag  = "--verbose"
)

// probePort is the port --web serves on, unless told otherwise, in a tree the
// tests build: workflow's root hands the kit the web server's default the same
// way.
const probePort = 4747

// errDirectoryGone is a working directory that cannot be named, as a shell is
// left standing in one when `git worktree remove` deletes the directory it is
// in.
var errDirectoryGone = errors.New("getwd: no such file or directory")

// probeTree is a command tree declared as workflow's root declares its own —
// the root's flags, env handed over before any command runs, and every mistake
// in a call marked as usage — whose root and one subcommand, probe, both run
// run.
func probeTree(env cmdkit.Environment, run func(cmd *cobra.Command, args []string) error) *cobra.Command {
	var flags cmdkit.RootFlags

	root := &cobra.Command{
		Use: "workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return cmdkit.HandOver(cmd, env) },
		RunE:              run,
	}

	flags.Declare(root, probePort)
	root.AddCommand(&cobra.Command{Use: probeCommand, Args: cobra.ArbitraryArgs, RunE: run})
	cmdkit.MarkMisuse(root)

	return root
}

// runTree runs root with args under t's context, and returns what it printed,
// stream by stream.
func runTree(t *testing.T, root *cobra.Command, args ...string) (clitest.Streams, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root.SetArgs(args)
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.ExecuteContext(t.Context())

	//nolint:wrapcheck // the tree's own error, as the command line it stands for says it
	return clitest.Streams{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

// inATempDir is an environment run from a directory of t's own, with a home of
// its own, and that directory.
func inATempDir(t *testing.T) (cmdkit.Environment, string) {
	t.Helper()

	dir := t.TempDir()

	return clitest.EnvironmentFor(t, clitest.Place{Dir: dir, Home: t.TempDir()}), dir
}

// fromARemovedDirectory is an environment whose working directory cannot be
// named.
func fromARemovedDirectory(t *testing.T) cmdkit.Environment {
	t.Helper()

	env, _ := inATempDir(t)
	env.WorkingDir = func() (string, error) { return "", errDirectoryGone }

	return env
}
