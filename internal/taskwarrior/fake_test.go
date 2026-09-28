// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

var errUnanswered = errors.New("the fake task program has no answer for this command")

// What the fake task program is called, the command words it answers, and
// the answers many tests need: the version it reports, an export that matched
// nothing, go-task's answer to _version, and a Taskwarrior never run.
const (
	taskProgram   = "task"
	versionWord   = "_version"
	showWord      = "_show"
	exportWord    = "export"
	versionAnswer = taskwarrior.MinimumVersion + "\n"
	noTasks       = "[\n]\n"
	goTask        = "gotask"
	goTaskAnswer  = `task: Task "_version" does not exist`
	noRCFile      = "Cannot proceed without rc file."
)

// reply is what the fake task program answers one command with.
type reply struct {
	stdout string
	err    error
}

// call is one run the fake task program saw, the directory it ran in, and
// whether its context was still live when it ran.
type call struct {
	program string
	args    []string
	dir     string
	timeout time.Duration
	live    bool
}

// fakeTask is a task program answering from a table keyed on the command word,
// or on the program and its command word ("/x/task _version") where two
// programs answer the same command differently. It records every run.
type fakeTask struct {
	replies map[string]reply
	calls   []call
}

// answering is a fake task program that answers one command word with stdout.
func answering(word, stdout string) *fakeTask {
	return &fakeTask{replies: map[string]reply{word: {stdout: stdout}}}
}

// run is the fake's taskwarrior.Runner. Like proc.CaptureWithin, it answers a
// run its context ended with the context's cause, never with an exit.
func (f *fakeTask) run(ctx context.Context, timeout time.Duration, program proc.Command, _ []byte) ([]byte, error) {
	f.calls = append(f.calls, call{
		program: program.Name, args: slices.Clone(program.Args), dir: program.Dir, timeout: timeout,
		live: ctx.Err() == nil,
	})

	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s: %w", program.Name, context.Cause(ctx))
	}

	for _, arg := range program.Args {
		for _, key := range []string{program.Name + " " + arg, arg} {
			if answer, ok := f.replies[key]; ok {
				return []byte(answer.stdout), answer.err
			}
		}
	}

	return nil, errUnanswered
}

// cancelingAfter is the fake's Runner that calls cancel once a run whose
// arguments hold word has answered, as a Ctrl-C between two runs would; with
// no word it never does.
func (f *fakeTask) cancelingAfter(word string, cancel context.CancelFunc) taskwarrior.Runner {
	return func(ctx context.Context, timeout time.Duration, program proc.Command, input []byte) ([]byte, error) {
		out, err := f.run(ctx, timeout, program, input)
		if word != "" && slices.Contains(program.Args, word) {
			cancel()
		}

		return out, err
	}
}

// liveRuns counts the runs that saw their context still live.
func liveRuns(calls []call) int {
	live := 0

	for _, run := range calls {
		if run.live {
			live++
		}
	}

	return live
}

// client drives the fake as an installed Taskwarrior with a sync backend.
func (f *fakeTask) client() taskwarrior.Client {
	return taskwarrior.New(f.run, taskwarrior.Install{
		Program: taskProgram, Version: taskwarrior.MinimumVersion, SyncConfigured: true,
	})
}

// checkRuns fails t unless got is exactly want, program and arguments alike.
func checkRuns(t *testing.T, got, want []call) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("ran %d commands %+v, want %d: %+v", len(got), got, len(want), want)
	}

	for index, run := range got {
		if run.program != want[index].program || !slices.Equal(run.args, want[index].args) {
			t.Errorf("run %d = %s %q, want %s %q", index, run.program, run.args, want[index].program, want[index].args)
		}
	}
}

// exited is how a task program's non-zero exit comes back from proc: a real
// program that wrote stderr and exited with code, so proc.Failure reads it
// exactly as it reads Taskwarrior's.
func exited(t *testing.T, code int, stderr string) error {
	t.Helper()

	_, err := proc.CaptureWithin(t.Context(), time.Minute, proc.Command{
		Name: "sh",
		Args: []string{"-c", `printf '%s' "$1" >&2; exit "$2"`, "sh", stderr, strconv.Itoa(code)},
	}, nil)
	if err == nil {
		t.Fatalf("sh exiting %d returned no error", code)
	}

	return err
}

// linux is the system Candidates is asked about where a file's execute bit
// counts.
const linux = "linux"

// What a file on the PATH list is when it is not a plain file.
const (
	directory = iota + 1
	symlink
)

// place puts a program of kind at path: a plain file or a directory with mode,
// or a link to an executable file elsewhere.
func place(t *testing.T, path string, kind int, mode os.FileMode) {
	t.Helper()

	var err error

	switch kind {
	case directory:
		err = os.Mkdir(path, mode)
	case symlink:
		target := filepath.Join(t.TempDir(), "taskwarrior")
		err = errors.Join(os.WriteFile(target, nil, 0o755), os.Symlink(target, path))
	default:
		err = os.WriteFile(path, nil, mode)
	}

	if err != nil {
		t.Fatal(err)
	}
}
