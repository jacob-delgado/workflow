// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/jacob-delgado/workflow/internal/proc/pgroup"
)

// maxLine is the longest line Start delivers whole. A longer one — a minified
// file echoed by a linter, say — fails the scan rather than growing a buffer
// without bound.
const maxLine = 1 << 20

// lineBuffer is how many lines may wait for a reader before the program is made
// to wait instead.
const lineBuffer = 64

// Command is a program to run, and where and how to run it.
type Command struct {
	// Dir is the working directory.
	Dir  string
	Name string
	Args []string
	// Env is added to this process's own environment rather than replacing it:
	// a git hook needs PATH, HOME and the rest to work at all.
	Env []string
}

// Output is a program's combined output as it arrives, and how it ended.
type Output struct {
	// Lines delivers each line of standard output and standard error in the
	// order the program wrote them, and closes when the program has exited.
	Lines <-chan string
	// Wait reports how the program exited. It blocks until then, so call it
	// once Lines has closed, or to wait out a program nobody is reading. Calling
	// it again returns the same result rather than blocking.
	Wait func() error
	// Stop kills this run — the whole process group — without disturbing the
	// context the caller passed, so one hung or unwanted run can be ended on its
	// own. Wait then reports the program as killed. Calling it more than once, or
	// after the program has already exited, does nothing.
	Stop func()
}

// Start runs a program and streams its output, for work long enough that
// someone should see it as it happens: a hook, a push.
//
// Standard output and standard error share one pipe, so a failure appears
// between the lines it followed rather than all at the end. Canceling ctx kills
// the program, and the output is drained whether or not anyone is reading, so a
// reader that goes away never leaves the program blocked on a full pipe.
func Start(ctx context.Context, program Command) (Output, error) {
	// A run gets its own cancelable context, a child of the caller's, so Stop can
	// end this run alone while the caller's context — and every other run under
	// it — carries on. Canceling the caller's context still reaches this child.
	runCtx, cancel := context.WithCancel(ctx)

	// A run that never starts leaks its context; a run that does owns it, and the
	// reader releases it when the program exits. started tells the two apart.
	started := false

	defer func() {
		if !started {
			cancel()
		}
	}()

	command, err := build(runCtx, program)
	if err != nil {
		return Output{}, err
	}

	// A streamed program is the kind that spawns children of its own — a push
	// runs ssh, a hook runs whatever it likes — so canceling the context kills
	// the whole group, not the child alone. Run and Capture are quick reads that
	// do not.
	pgroup.Isolate(command)

	reader, writer, err := os.Pipe()
	if err != nil {
		return Output{}, fmt.Errorf("%s: opening a pipe: %w", program.Name, err)
	}

	command.Stdout, command.Stderr = writer, writer

	err = command.Start()
	// The child holds its own copy of the write end. Closing this one is what
	// lets the reader see the end of the output when the child exits.
	_ = writer.Close()

	if err != nil {
		_ = reader.Close()

		return Output{}, fmt.Errorf("%s: %w", program.Name, err)
	}

	lines := make(chan string, lineBuffer)
	done := make(chan error, 1)

	go func() {
		scanErr := deliver(runCtx, reader, lines)

		close(lines)

		_ = reader.Close()

		result := exited(program.Name, command.Wait(), scanErr)

		// The program has been reaped; releasing the context now frees it whether
		// or not Stop was ever called, without disturbing that result or firing
		// the group kill on a run that ended on its own.
		cancel()

		done <- result
	}()

	started = true

	// OnceValue so a second Wait returns the same result rather than blocking on
	// a channel the first call already drained.
	return Output{Lines: lines, Wait: sync.OnceValue(func() error { return <-done }), Stop: cancel}, nil
}

// deliver sends each line until the output ends, and never leaves the program
// writing to a pipe that nobody empties: once ctx is done it keeps reading
// without sending, and after a line too long to scan it discards the rest.
func deliver(ctx context.Context, output io.Reader, lines chan<- string) error {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLine)

	for scanner.Scan() {
		select {
		case lines <- scanner.Text():
		case <-ctx.Done():
		}
	}

	err := scanner.Err()
	if err != nil {
		_, _ = io.Copy(io.Discard, output)

		return fmt.Errorf("reading its output: %w", err)
	}

	return nil
}

// exited describes how a program ended. The exit comes first: a program that
// failed matters more than a line too long to show.
func exited(name string, waitErr, scanErr error) error {
	switch {
	case waitErr != nil:
		return fmt.Errorf("%s: %w", name, waitErr)
	case scanErr != nil:
		return fmt.Errorf("%s: %w", name, scanErr)
	default:
		return nil
	}
}

// Interactive builds a program that takes over the terminal — an editor — for
// the interface to hand the screen to. It is not started here: Bubble Tea
// starts it once it has released the terminal.
func Interactive(program Command) (*exec.Cmd, error) {
	return build(context.Background(), program)
}

// build resolves and assembles a command.
func build(ctx context.Context, program Command) (*exec.Cmd, error) {
	path, err := exec.LookPath(program.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, program.Name)
	}

	//nolint:gosec // the program is this module's own choice or the user's $EDITOR, never remote input
	command := exec.CommandContext(ctx, path, program.Args...)
	command.Dir = program.Dir
	command.Env = append(os.Environ(), program.Env...)

	return command, nil
}
