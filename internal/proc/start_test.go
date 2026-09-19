// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

// TestMain makes this test binary the child a test starts when helperMode is
// set, and otherwise runs the tests. The child is dispatched here rather than
// from a test function, so it never shows up as a test that asserts nothing.
func TestMain(m *testing.M) {
	if mode := os.Getenv(helperMode); mode != "" {
		// Exits by itself: gobco rewrites an os.Exit written in TestMain to save
		// its counters, which every child would otherwise race to overwrite.
		actAsHelper(mode)
	}

	os.Exit(m.Run())
}

// actAsHelper is the child: it does what mode asks, then exits.
func actAsHelper(mode string) {
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
	case "grandchild":
		// Spawn a long-lived grandchild — another copy of this binary, sleeping —
		// print its pid, then block, so a test can cancel the run and check the
		// grandchild died with its parent rather than outliving it.
		//nolint:noctx // this test binary re-run as the sleeper above, in a helper that has no context to thread
		grand := exec.Command(os.Args[0], "-test.run=^$")

		grand.Env = append(os.Environ(), helperMode+"=sleep")

		_ = grand.Start()

		fmt.Fprintln(os.Stdout, grand.Process.Pid)

		_ = grand.Wait()
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

// child describes a run of this test binary as the helper, in a mode. It runs
// no tests, so a child that somehow missed its mode does nothing.
func child(dir, mode string, env ...string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: os.Args[0],
		Args: []string{"-test.run=^$"},
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

func TestWaitReturnsTheSameResultWhenCalledAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// "mixed" exits 3, so Wait returns a non-nil error the first time.
	output, err := proc.Start(t.Context(), child(t.TempDir(), "mixed"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	_, first := collect(t, output)

	// Act
	// A second Wait must return the same result, not block on a drained channel.
	second := output.Wait()

	// Assert
	if first == nil || second == nil || first.Error() != second.Error() {
		t.Errorf("second Wait = %v, want the same error as the first (%v)", second, first)
	}
}

func TestStartStreamsBothStreamsInOrderAndReportsTheExit(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.Start(t.Context(), child(t.TempDir(), "mixed"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	lines, err := collect(t, output)

	// Assert
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

func TestStartRunsInTheDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	output, err := proc.Start(t.Context(), child(dir, "pwd"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	lines, err := collect(t, output)
	if err != nil {
		t.Fatalf("Wait returned %v, want nil", err)
	}

	// Assert
	// Temporary directories are reached through a symlink on macOS.
	want, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(strings.Join(lines, "")); got != want {
		t.Errorf("ran in %q, want %q", got, want)
	}
}

func TestStartPassesTheEnvironment(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.Start(t.Context(), child(t.TempDir(), "env", "WORKFLOW_PROC_VALUE=forty-two"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	lines, err := collect(t, output)

	// Assert
	if err != nil || !slices.Equal(lines, []string{"forty-two"}) {
		t.Errorf("the child saw %q and ended %v, want the value passed on", lines, err)
	}
}

func TestALineTooLongToShowIsReportedWithoutKillingTheProgram(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.Start(t.Context(), child(t.TempDir(), "long"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	finished := make(chan error, 1)

	go func() {
		_, waitErr := collect(t, output)
		finished <- waitErr
	}()

	// Assert
	// Were the rest of the output not drained, the program would die of
	// SIGPIPE writing its next line — a push or a commit killed partway through
	// for printing something long.
	select {
	case err := <-finished:
		if err == nil || !strings.Contains(err.Error(), "too long") {
			t.Errorf("Wait returned %v, want the over-long line reported and the program left to finish", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the program never finished")
	}
}

func TestStartReportsWhatItCannotStart(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		dirName string
		program string
		want    error
	}{
		"a program not on PATH":           {dirName: "", program: missingProgram, want: proc.ErrNotFound},
		"a directory that does not exist": {dirName: "gone", program: os.Args[0], want: fs.ErrNotExist},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			command := proc.Command{Dir: filepath.Join(t.TempDir(), tt.dirName), Name: tt.program, Args: nil, Env: nil}

			// Act
			_, err := proc.Start(t.Context(), command)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Start returned %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCancelingEndsTheProgramEvenWhenNobodyReads(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())

	output, err := proc.Start(ctx, child(t.TempDir(), "sleep"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	// Act
	cancel()

	finished := make(chan error, 1)

	go func() { finished <- output.Wait() }()

	// Assert
	select {
	case err := <-finished:
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok || exitErr.Exited() {
			t.Errorf("Wait returned %v, want the program killed rather than left to exit", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the program outlived its context")
	}
}

func TestStopEndsARunningProgramWithoutCancelingTheCaller(t *testing.T) {
	t.Parallel()

	// Arrange
	// The caller's context stays alive; only this run's own Stop is used, so a
	// hung run can be ended without tearing down everything under that context.
	output, err := proc.Start(t.Context(), child(t.TempDir(), "sleep"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	// Act
	output.Stop()

	finished := make(chan error, 1)

	go func() { finished <- output.Wait() }()

	// Assert
	select {
	case err := <-finished:
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok || exitErr.Exited() {
			t.Errorf("Wait returned %v, want the program killed rather than left to exit", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop did not end the program")
	}
}

func TestInteractiveBuildsACommandWithoutStartingIt(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	editor := proc.Command{Dir: dir, Name: goProgram, Args: []string{versionSubcommand}, Env: []string{"A=B"}}

	// Act
	command, err := proc.Interactive(editor)
	if err != nil {
		t.Fatalf("Interactive returned %v, want nil", err)
	}

	// Assert
	if command.Process != nil {
		t.Error("Interactive started the program, want it left for the caller")
	}

	if command.Dir != dir || !slices.Equal(command.Args[1:], []string{versionSubcommand}) ||
		!slices.Contains(command.Env, "A=B") {
		t.Errorf("Interactive built %v in %q with %d environment entries", command.Args, command.Dir, len(command.Env))
	}
}

func TestInteractiveReportsAProgramNotOnPath(t *testing.T) {
	t.Parallel()

	// Act
	_, err := proc.Interactive(proc.Command{Dir: t.TempDir(), Name: missingProgram, Args: nil, Env: nil})

	// Assert
	if !errors.Is(err, proc.ErrNotFound) {
		t.Errorf("Interactive returned %v, want ErrNotFound", err)
	}
}
