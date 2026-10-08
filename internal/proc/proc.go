// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package proc runs external programs.
//
// It is the only place in this module that spawns a subprocess. Concentrating
// that here means the gosec exception for a program name held in a variable is
// written once, with its reason, instead of being repeated at every call site
// where it would gradually stop being read.
package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// ErrNotFound reports a program that is not on PATH.
var ErrNotFound = errors.New("program not found on PATH")

// ErrTimedOut reports a Run stopped because it outlasted its own bound; the
// error names the bound. A run the caller's context ended first — its own
// deadline, a Ctrl-C — does not answer to it.
var ErrTimedOut = errors.New("gave up waiting")

// ExitError is a program that ran and ended with a failure status of its own:
// which program, the status, and what it wrote to standard error, sanitized.
type ExitError struct {
	Program string
	Code    int
	Stderr  string
}

var _ error = (*ExitError)(nil)

// Error is the program, its status and its reason, as git or gh would put it.
func (e *ExitError) Error() string {
	if e.Stderr == "" {
		return fmt.Sprintf("%s: exit status %d", e.Program, e.Code)
	}

	return fmt.Sprintf("%s: exit status %d: %s", e.Program, e.Code, e.Stderr)
}

// DefaultRunTimeout bounds a Run so a hung quick read — a git status on a dead
// network mount, a gh call to a host that never answers — recovers on its own,
// with an error answering to ErrTimedOut, rather than leaving a pane on
// "loading…" forever. It is generous because a Run is a quick read; streamed
// work (Start) and piped work (Capture) can legitimately run long and are not
// bounded here.
const DefaultRunTimeout = 30 * time.Second

// Run executes name with args and returns what it wrote to standard output,
// within DefaultRunTimeout.
//
// A non-zero exit becomes an error carrying the program's standard error. That
// matters more than it looks: "git failed" without the reason sends the reader
// back to a terminal to run the command again by hand.
func Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return RunWithin(ctx, DefaultRunTimeout, name, args...)
}

// RunWithin is Run bounded by an explicit timeout, for a read whose own limit
// differs from the default. The tighter of timeout and ctx's own deadline wins;
// only timeout's own expiry answers to ErrTimedOut.
func RunWithin(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return runWithin(ctx, timeout, Command{Name: name, Args: args})
}

// RunCommand is Run for a caller that must set the working directory or the
// environment: a git command that touches the network with GIT_TERMINAL_PROMPT
// off, so it fails at once rather than block on a credential prompt nobody
// inside the interface can answer. It is bounded by DefaultRunTimeout like Run.
func RunCommand(ctx context.Context, program Command) ([]byte, error) {
	return runWithin(ctx, DefaultRunTimeout, program)
}

// runWithin runs a program bounded by timeout and returns its standard output,
// or nothing when it fails. The tighter of timeout and ctx's own deadline wins.
func runWithin(ctx context.Context, timeout time.Duration, program Command) ([]byte, error) {
	output, err := captureWithin(ctx, timeout, program, nil)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// Capture runs a program with input on its standard input and returns what it
// wrote to standard output.
//
// Unlike Run it feeds a body in, and it returns the output even on a non-zero
// exit: a tool that answers with data on standard output and signals an HTTP
// error only through its exit status — gh api, glab api — is read either way,
// with its standard error folded into the returned error.
func Capture(ctx context.Context, program Command, input []byte) ([]byte, error) {
	return captureWithin(ctx, 0, program, input)
}

// CaptureWithin is Capture bounded by timeout, as RunWithin bounds Run; a
// timeout that is not positive leaves it unbounded, as Capture is, and only
// timeout's own expiry answers to ErrTimedOut.
func CaptureWithin(ctx context.Context, timeout time.Duration, program Command, input []byte) ([]byte, error) {
	return captureWithin(ctx, timeout, program, input)
}

// captureWithin is Capture bounded by timeout, or unbounded when timeout is
// not positive.
//
// The bound travels as the context's cause because Wait reports the killed
// process's own "signal: killed" over the context's error; the cause is what
// tells a run this bound stopped from one the caller's context ended, whose own
// error, context.Canceled or context.DeadlineExceeded, is returned instead.
func captureWithin(ctx context.Context, timeout time.Duration, program Command, input []byte) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeoutCause(ctx, timeout, fmt.Errorf("%w after %s", ErrTimedOut, timeout))
		defer cancel()
	}

	command, err := build(ctx, program)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer

	command.Stdout, command.Stderr = &stdout, &stderr

	if len(input) > 0 {
		command.Stdin = bytes.NewReader(input)
	}

	err = command.Run()
	if err != nil {
		return stdout.Bytes(), failed(ctx, program.Name, err, &stderr)
	}

	return stdout.Bytes(), nil
}

// failed is how a run that returned err ended: the context's cause once ctx is
// done, an ExitError for a program that exited with a status of its own, and
// otherwise exec's error, a signal say, with what the program wrote to stderr.
func failed(ctx context.Context, name string, err error, stderr *bytes.Buffer) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", name, context.Cause(ctx))
	}

	reason := strings.TrimSpace(sanitize.Text(stderr.String()))

	exit, ok := errors.AsType[*exec.ExitError](err)
	if ok && exit.Exited() {
		return &ExitError{Program: name, Code: exit.ExitCode(), Stderr: reason}
	}

	return fmt.Errorf("%s: %w: %s", name, err, reason)
}

// Failure reads a program's non-zero exit out of an error Run, RunCommand or
// Capture returned: its exit code and what it wrote to standard error, and
// whether the error was an exit at all — a program not found, one stopped at
// its bound, or one killed by a signal is not.
func Failure(err error) (int, string, bool) {
	exit, ok := errors.AsType[*ExitError](err)
	if !ok {
		return 0, "", false
	}

	return exit.Code, exit.Stderr, true
}

// LookPath reports where a program is, or an error if it is not on PATH. It is
// exec.LookPath, re-exported so callers wiring a seam do not reach past this
// package for one of its two halves.
func LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	return path, nil
}

// Available reports whether name can be found on PATH.
func Available(name string) bool {
	_, err := exec.LookPath(name)

	return err == nil
}
