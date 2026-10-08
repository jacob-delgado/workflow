// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc/pgroup"
)

// maxLine is the longest line Start delivers whole. A longer one — a minified
// file echoed by a linter, say — fails the scan rather than growing a buffer
// without bound.
const maxLine = 1 << 20

// lineBuffer is how many lines may wait for a reader before the program is made
// to wait instead.
const lineBuffer = 64

// outputGrace is how long a run's output may keep it waiting once the program
// has exited. A process the program left behind — a server a hook backgrounded,
// a daemon that left the group — holds the pipe open long after, so past the
// grace the pipe is closed and the run ends. It counts only the time spent
// waiting on the pipe, never on whoever reads Lines, so a slow reader still
// gets every line the program wrote.
const outputGrace = 2 * time.Second

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
	run := stream{
		name: program.Name, command: command, reader: reader,
		sender: &lineSender{mu: sync.Mutex{}, lines: lines, closed: false, abandoned: make(chan struct{})},
	}

	go func() {
		result := run.follow(runCtx)

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

// stream is one streamed run as its reader follows it.
type stream struct {
	name    string
	command *exec.Cmd
	reader  *os.File
	sender  *lineSender
}

// follow delivers the run's output until it ends, or until the program has
// exited and the pipe has kept it waiting outputGrace, and reports how the
// program ended. The program is waited for on its own, since a process it
// left behind can hold the pipe open past its exit.
func (s stream) follow(ctx context.Context) error {
	waited := make(chan error, 1)

	go func() {
		waited <- s.command.Wait()
	}()

	onPipe := make(chan bool)
	scanned := make(chan error, 1)

	go func() {
		scanned <- deliver(ctx, s.reader, s.sender, s.waiting(onPipe))
	}()

	waitErr, scanErr := s.watch(waited, onPipe, scanned)

	s.sender.close()

	_ = s.reader.Close()

	return exited(s.name, waitErr, scanErr)
}

// waiting is how deliver says it is, or is no longer, waiting on the pipe,
// which stops mattering once the run has abandoned its output.
func (s stream) waiting(onPipe chan<- bool) func(bool) {
	return func(waiting bool) {
		select {
		case onPipe <- waiting:
		case <-s.sender.abandoned:
		}
	}
}

// watch follows the run to the end of its output, counting outputGrace down
// while the program has exited and deliver waits on the pipe. Past it the
// pipe is closed, which ends a read in progress on Unix, and the output is
// abandoned, so the run ends even where that read cannot be ended.
func (s stream) watch(waited <-chan error, onPipe <-chan bool, scanned <-chan error) (error, error) {
	var (
		waitErr error
		reading bool
	)

	grace := pipeGrace{left: outputGrace, started: time.Time{}, timer: nil}
	exit := waited

	for {
		select {
		case reading = <-onPipe:
			grace.run(exit == nil && reading)
		case waitErr = <-exit:
			exit = nil

			grace.run(reading)
		case scanErr := <-scanned:
			if exit != nil {
				waitErr = <-exit
			}

			return waitErr, scanErr
		case <-grace.expired():
			s.sender.abandon()

			_ = s.reader.Close()

			return waitErr, nil
		}
	}
}

// pipeGrace counts a duration down only while it runs, so the time a reader
// spends waiting on someone else is not counted against it.
type pipeGrace struct {
	left    time.Duration
	started time.Time
	timer   *time.Timer
}

// run starts or pauses the count.
func (g *pipeGrace) run(running bool) {
	switch {
	case running && g.timer == nil:
		g.started = time.Now()
		g.timer = time.NewTimer(g.left)
	case !running && g.timer != nil:
		g.timer.Stop()
		g.left -= time.Since(g.started)
		g.timer = nil
	}
}

// expired delivers once the count reaches zero, and never while it is paused.
func (g *pipeGrace) expired() <-chan time.Time {
	if g.timer == nil {
		return nil
	}

	return g.timer.C
}

// lineSender hands lines to Lines until it is closed, and drops any after, so
// Lines can close while a read the run abandoned is still in progress.
type lineSender struct {
	mu     sync.Mutex
	lines  chan string
	closed bool
	// abandoned closes when the run stops waiting for the rest of its output, so
	// a send or a read still in progress delivers nothing more.
	abandoned chan struct{}
}

// send hands a line on, unless ctx is done or the output is abandoned or
// closed, so the program is never left writing to a pipe nobody empties.
func (s *lineSender) send(ctx context.Context, line string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	select {
	case s.lines <- line:
	case <-ctx.Done():
	case <-s.abandoned:
	}
}

// abandon gives up on the rest of the output, ending a send in progress.
func (s *lineSender) abandon() {
	close(s.abandoned)
}

// close closes Lines, once.
func (s *lineSender) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closed {
		s.closed = true
		close(s.lines)
	}
}

// deliver sends each line until the output ends, and never leaves the program
// writing to a pipe that nobody empties: once ctx is done it keeps reading
// without sending, and after a line too long to scan it discards the rest. It
// says through waiting when it starts and stops waiting on the pipe.
func deliver(ctx context.Context, output io.Reader, sender *lineSender, waiting func(bool)) error {
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLine)

	for {
		waiting(true)

		more := scanner.Scan()

		waiting(false)

		if !more {
			break
		}

		sender.send(ctx, scanner.Text())
	}

	err := scanner.Err()
	if err != nil {
		_, _ = io.Copy(io.Discard, output)

		return fmt.Errorf("reading its output: %w", err)
	}

	return nil
}

// ErrExitStatus reports a program that ran and ended with a failure status of
// its own — a refused commit, a rejected push — rather than one that could not
// start, was killed, or could not be read. The error keeps the program's own
// words, "git: exit status 1", so it reads as it always has.
var ErrExitStatus = errors.New("ended with a failure status")

// exitStatusError is a failure status in the process's own words, answering to
// ErrExitStatus as well as to the *exec.ExitError it carries.
type exitStatusError struct {
	exit error
}

var _ error = exitStatusError{}

// Error is the status as the process put it.
func (e exitStatusError) Error() string {
	return e.exit.Error()
}

// Unwrap is both what the status is and the process's error beneath it.
func (e exitStatusError) Unwrap() []error {
	return []error{ErrExitStatus, e.exit}
}

// exitedWithStatus reports a program that exited by itself with a failure
// status, as against one a signal ended.
func exitedWithStatus(waitErr error) bool {
	exit, ok := errors.AsType[*exec.ExitError](waitErr)

	return ok && exit.ExitCode() > 0
}

// exited describes how a program ended. The exit comes first: a program that
// failed matters more than a line too long to show.
func exited(name string, waitErr, scanErr error) error {
	switch {
	case exitedWithStatus(waitErr):
		return fmt.Errorf("%s: %w", name, exitStatusError{exit: waitErr})
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
