// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package clitest runs the workflow command line for a test, as a user or a
// script would type it: each run in a world of its own — a working directory,
// a home and none of the developer's environment but what the test set — with
// a Jira that answers, and the process readied for the tests by Main. It is
// the support the command tree's packages share for their black-box tests;
// only tests import it.
package clitest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errUnexpectedPrompt is a command reading from the prompt when the test wired
// none.
var errUnexpectedPrompt = errors.New("unexpected prompt read")

// Run executes the command tree in dir and returns everything it printed.
//
// Each run is handed an Environment of its own, run from dir with an empty home
// of its own and none of the developer's environment, so the tests run side by
// side: see EnvironmentFor and IsolatedEnvironment.
func Run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	return RunGuided(t, dir, UnusedPrompt(t), args...)
}

// RunGuided is Run with a prompt that answers `config init`'s questions.
func RunGuided(t *testing.T, dir string, prompt cmdkit.Prompt, args ...string) (string, error) {
	t.Helper()

	printed, err := RunStreams(t, dir, prompt, args...)

	return printed.Stdout + printed.Stderr, err
}

// Streams is what a command printed, stream by stream.
type Streams struct {
	// Stdout is what the command printed on standard output.
	Stdout string
	// Stderr is what the command printed on standard error.
	Stderr string
}

// RunStreams is RunGuided with the two streams kept apart, for a test that
// says which one a line belongs on: the artifact on stdout, what is said about
// it on stderr.
func RunStreams(t *testing.T, dir string, prompt cmdkit.Prompt, args ...string) (Streams, error) {
	t.Helper()

	return RunStreamsAt(t, Place{Dir: dir, Home: t.TempDir()}, prompt, args...)
}

// Place is where a run happens: the working directory, and the home directory
// the test chose, for a test that puts something there first; state, when set,
// is $XDG_STATE_HOME, which is otherwise empty.
type Place struct {
	// Dir is the working directory the command runs from.
	Dir string
	// Home is the home directory the test chose.
	Home string
	// State, when set, is $XDG_STATE_HOME.
	State string
}

// RunStreamsAt is RunStreams in the home directory the test chose.
func RunStreamsAt(t *testing.T, where Place, prompt cmdkit.Prompt, args ...string) (Streams, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	err := cli.Execute(args, &stdout, &stderr, prompt, EnvironmentFor(t, where))

	return Streams{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

// IsolatedEnvironment is the environment every run gets, so nothing of the
// developer's own decides a result or is touched by one: the home is the
// test's — a ~/.workflow.json would stand in for a missing file — and the store
// is kept under it, in $XDG_STATE_HOME only where the test chose one, never in
// %AppData%, where the developer's own store is.
func IsolatedEnvironment(where Place) map[string]string {
	return map[string]string{
		"HOME":           where.Home,
		"XDG_STATE_HOME": where.State,
		"AppData":        filepath.Join(where.Home, "AppData"),
	}
}

// UnusedPrompt fails the test if a command reads from it: only the guided
// `config init` should, and its tests drive it with a scripted prompt instead.
func UnusedPrompt(t *testing.T) cmdkit.Prompt {
	t.Helper()

	fail := func(string) (string, error) {
		t.Error("a command read from the prompt when none was expected")

		return "", errUnexpectedPrompt
	}

	return cmdkit.Prompt{Line: fail, Secret: fail}
}

// WriteFile writes a configuration into dir, failing the test if it cannot.
func WriteFile(t *testing.T, dir, contents string) string {
	t.Helper()

	path := filepath.Join(dir, config.FileName)

	err := os.WriteFile(path, []byte(contents), config.FileMode)
	if err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	return path
}

// ReadLog is what the request log at path holds, failing the test if it
// cannot be read.
func ReadLog(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path) //nolint:gosec // the log the test named, in a directory it made
	if err != nil {
		t.Fatalf("reading the request log: %v", err)
	}

	return string(contents)
}
