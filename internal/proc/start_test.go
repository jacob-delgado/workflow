// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// helperMode is the environment variable that turns this test binary into the
// child process a test starts. Re-running the test binary is the standard way
// to spawn a real process with exactly the behavior a test needs, on any
// platform, without depending on a shell.
const helperMode = "WORKFLOW_PROC_HELPER"

// TestHelperProcess is not a test: it is the child. It does nothing unless
// started by one of the tests below.
func TestHelperProcess(t *testing.T) {
	t.Parallel()

	mode := os.Getenv(helperMode)
	if mode == "" {
		return
	}

	switch mode {
	case "mixed":
		fmt.Fprintln(os.Stdout, "one")
		fmt.Fprintln(os.Stderr, "two")
		fmt.Fprintln(os.Stdout, "three")
		os.Exit(3)
	case "pwd":
		dir, _ := os.Getwd()
		fmt.Fprintln(os.Stdout, dir)
	case "env":
		fmt.Fprintln(os.Stdout, os.Getenv("WORKFLOW_PROC_VALUE"))
	case "sleep":
		time.Sleep(time.Minute)
	case "long":
		// A line past what Start will deliver, then more output the program
		// must still be able to write.
		fmt.Fprintln(os.Stdout, strings.Repeat("x", 2<<20))

		for range 1000 {
			fmt.Fprintln(os.Stdout, "after")
		}
	}

	os.Exit(0)
}

// child describes a run of this test binary as the helper, in a mode.
func child(dir, mode string, env ...string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: os.Args[0],
		Args: []string{"-test.run=^TestHelperProcess$"},
		Env:  append([]string{helperMode + "=" + mode}, env...),
	}
}

// collect reads every line, then how the program ended.
func collect(t *testing.T, output proc.Output) ([]string, error) {
	t.Helper()

	var lines []string

	for line := range output.Lines {
		lines = append(lines, line)
	}

	return lines, output.Wait()
}

func TestStartStreamsBothStreamsInOrderAndReportsTheExit(t *testing.T) {
	t.Parallel()

	output, err := proc.Start(t.Context(), child(t.TempDir(), "mixed"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	lines, err := collect(t, output)

	// Standard error is interleaved where it was written: a hook's failure is
	// read in the context of what came before it.
	if want := []string{"one", "two", "three"}; !slices.Equal(lines, want) {
		t.Errorf("lines = %q, want %q", lines, want)
	}

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 3 {
		t.Errorf("Wait returned %v, want exit status 3", err)
	}
}

func TestStartRunsInTheDirectoryWithTheEnvironment(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	output, err := proc.Start(t.Context(), child(dir, "pwd"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	lines, err := collect(t, output)
	if err != nil {
		t.Fatalf("Wait returned %v, want nil", err)
	}

	// Temporary directories are reached through a symlink on macOS.
	want, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(strings.Join(lines, "")); got != want {
		t.Errorf("ran in %q, want %q", got, want)
	}

	output, err = proc.Start(t.Context(), child(dir, "env", "WORKFLOW_PROC_VALUE=forty-two"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	if lines, _ := collect(t, output); !slices.Equal(lines, []string{"forty-two"}) {
		t.Errorf("the environment was not passed on: %q", lines)
	}
}

func TestALineTooLongToShowIsReportedWithoutKillingTheProgram(t *testing.T) {
	t.Parallel()

	output, err := proc.Start(t.Context(), child(t.TempDir(), "long"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	// Were the rest of the output not drained, the program would die of
	// SIGPIPE writing its next line — a push or a commit killed partway through
	// for printing something long.
	finished := make(chan error, 1)

	go func() {
		_, waitErr := collect(t, output)
		finished <- waitErr
	}()

	select {
	case err := <-finished:
		if err == nil || !strings.Contains(err.Error(), "too long") {
			t.Errorf("Wait returned %v, want the over-long line reported and the program left to finish", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the program never finished")
	}
}

func TestStartReportsAProgramNotOnPath(t *testing.T) {
	t.Parallel()

	_, err := proc.Start(t.Context(), proc.Command{Dir: t.TempDir(), Name: missingProgram, Args: nil, Env: nil})
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Start returned %v, want ErrNotFound", err)
	}
}

func TestStartReportsADirectoryThatDoesNotExist(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "gone")

	_, err := proc.Start(t.Context(), child(missing, "pwd"))
	if err == nil {
		t.Error("Start returned nil for a directory that does not exist")
	}
}

func TestCancelingEndsTheProgramEvenWhenNobodyReads(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	output, err := proc.Start(ctx, child(t.TempDir(), "sleep"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	cancel()

	finished := make(chan error, 1)

	go func() { finished <- output.Wait() }()

	select {
	case err := <-finished:
		if err == nil {
			t.Error("Wait returned nil for a program that was killed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the program outlived its context")
	}
}

func TestInteractiveBuildsACommandWithoutStartingIt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	editor := proc.Command{Dir: dir, Name: goProgram, Args: []string{"version"}, Env: []string{"A=B"}}

	command, err := proc.Interactive(editor)
	if err != nil {
		t.Fatalf("Interactive returned %v, want nil", err)
	}

	if command.Process != nil {
		t.Error("Interactive started the program, want it left for the caller")
	}

	if command.Dir != dir || !slices.Equal(command.Args[1:], []string{"version"}) || !slices.Contains(command.Env, "A=B") {
		t.Errorf("Interactive built %v in %q with %d environment entries", command.Args, command.Dir, len(command.Env))
	}

	_, err = proc.Interactive(proc.Command{Dir: dir, Name: missingProgram, Args: nil, Env: nil})
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Interactive returned %v, want ErrNotFound", err)
	}
}
