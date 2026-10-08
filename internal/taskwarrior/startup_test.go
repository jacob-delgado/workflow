// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// foundTaskwarrior is where a test's Taskwarrior is found, after go-task.
const foundTaskwarrior = "/usr/local/bin/task"

func TestDetectProbesFromTheFilesystemRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	// go-task, also called task, runs a Taskfile's _version task from the
	// directory it starts in, or any directory above it.
	fake := &fakeTask{replies: map[string]reply{
		goTask + " _version":           {err: exited(t, 200, goTaskAnswer)},
		foundTaskwarrior + " _version": {stdout: versionAnswer},
		showWord:                       {stdout: ""},
	}}

	// Act
	_, err := taskwarrior.Detect(t.Context(), "", []string{goTask, foundTaskwarrior}, fake.run)
	// Assert
	if err != nil {
		t.Fatalf("Detect returned %v", err)
	}

	root := string(filepath.Separator)
	for _, run := range fake.calls {
		if run.dir != root {
			t.Errorf("Detect ran %s %q in %q, want the filesystem root %q", run.program, run.args, run.dir, root)
		}
	}

	if len(fake.calls) != 3 {
		t.Errorf("Detect ran %+v, want go-task's _version, then Taskwarrior's _version and _show", fake.calls)
	}
}

func TestDetectNeverRunsAProgramRelativeToTheWorkingDirectory(t *testing.T) {
	t.Parallel()

	for _, relative := range []string{filepath.Join("bin", taskProgram), "." + string(filepath.Separator) + taskProgram} {
		t.Run(relative, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{versionWord: {stdout: versionAnswer}, showWord: {stdout: ""}}}

			// Act
			_, err := taskwarrior.Detect(t.Context(), relative, nil, fake.run)

			// Assert
			if !errors.Is(err, taskwarrior.ErrRelativeProgram) || len(fake.calls) != 0 {
				t.Errorf("Detect = %v after running %+v; want ErrRelativeProgram and nothing run", err, fake.calls)
			}
		})
	}
}

func TestDetectStopsAtATaskwarriorThatCannotStart(t *testing.T) {
	t.Parallel()

	const broken = "/opt/homebrew/bin/task"

	tests := []struct {
		name   string
		stderr string
	}{
		{
			name:   "an include it cannot read",
			stderr: "Could not read include file '/opt/homebrew/Cellar/task/3.4.1/share/doc/task/rc/dark-256.theme'.",
		},
		{
			name: "a database it cannot open",
			stderr: "unable to open database file: /Users/x/.task/taskchampion.sqlite3: " +
				"Error code 14: unable to open database file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{
				broken + " _version":           {err: exited(t, 2, test.stderr)},
				foundTaskwarrior + " _version": {stdout: versionAnswer},
				showWord:                       {stdout: ""},
			}}

			// Act
			_, err := taskwarrior.Detect(t.Context(), "", []string{broken, foundTaskwarrior}, fake.run)

			// Assert
			if !errors.Is(err, taskwarrior.ErrRefused) || errors.Is(err, taskwarrior.ErrNotTaskwarrior) {
				t.Fatalf("Detect returned %v, want ErrRefused and not ErrNotTaskwarrior", err)
			}

			if !strings.Contains(err.Error(), test.stderr) {
				t.Errorf("Detect's error %q does not carry Taskwarrior's words %q", err, test.stderr)
			}

			if len(fake.calls) != 1 {
				t.Errorf("Detect ran %+v, want it to stop at the Taskwarrior that could not start", fake.calls)
			}
		})
	}
}

func TestDetectTellsGoTaskFromATaskwarriorThatCannotStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		code   int
		stderr string
	}{
		{name: "go-task with no such task", code: 200, stderr: goTaskAnswer},
		{
			name: "go-task with no Taskfile", code: 100,
			stderr: `task: No Taskfile found at "/" (or any of the parent directories).`,
		},
		{name: "go-task's words at exit 2", code: 2, stderr: goTaskAnswer},
		{name: "an exit 2 that says nothing", code: 2, stderr: ""},
		{name: "another program's words at exit 1", code: 1, stderr: "unknown command _version"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{
				goTask + " _version":           {err: exited(t, test.code, test.stderr)},
				foundTaskwarrior + " _version": {stdout: versionAnswer},
				showWord:                       {stdout: ""},
			}}

			// Act
			install, err := taskwarrior.Detect(t.Context(), "", []string{goTask, foundTaskwarrior}, fake.run)

			// Assert
			if err != nil || install.Program != foundTaskwarrior {
				t.Errorf("Detect = %+v, %v; want it to pass go-task by and keep %s", install, err, foundTaskwarrior)
			}
		})
	}
}
