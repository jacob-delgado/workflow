// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// A secret in a taskrc line Taskwarrior cannot parse, and the complaint
// Taskwarrior 3.5.0 prints for it on stderr, quoting the whole line.
const (
	taskrcSecret  = "hunter2"
	malformedLine = "Malformed entry 'sync.encryption_secret " + taskrcSecret + "' in config file."
)

func TestReadsNeverRepeatAMalformedTaskrcLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		read    string
		failing string
		stderr  string
	}{
		{name: pendingContextRun, read: pendingRead, failing: showWord, stderr: malformedLine},
		{name: pendingExportRun, read: pendingRead, failing: exportWord, stderr: malformedLine},
		{name: linkedExportRun, read: linkedRead, failing: exportWord, stderr: malformedLine},
		// The never-run line is told with the rest of stderr, so the malformed
		// one is told apart first.
		{
			name: "a malformed line beside the never-run one", read: pendingRead, failing: exportWord,
			stderr: malformedLine + "\n" + noRCFile,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: map[string]reply{showWord: {stdout: workContext}, exportWord: {stdout: noTasks}}}
			fake.replies[test.failing] = reply{err: exited(t, 2, test.stderr)}

			// Act
			err := reads()[test.read](t.Context(), fake.client())

			// Assert
			if !errors.Is(err, taskwarrior.ErrRefused) {
				t.Errorf("%s returned %v, want ErrRefused", test.read, err)
			}

			if seen := fmt.Sprint(err); strings.Contains(seen, taskrcSecret) {
				t.Errorf("%s repeated the taskrc secret: %s", test.read, seen)
			}
		})
	}
}

func TestWritesNeverRepeatAMalformedTaskrcLine(t *testing.T) {
	t.Parallel()

	each := writes()
	each[addWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Add(ctx, "Renew the cert")

		return err
	}
	each[undoWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Undo(ctx)

		return err
	}
	each[syncWord] = func(ctx context.Context, client taskwarrior.Client) error {
		_, err := client.Sync(ctx)

		return err
	}

	// Exit 1 is otherwise a write declined, told in fixed words of its own; a
	// malformed line is refused whatever the exit.
	for verb, write := range each {
		for _, code := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s exiting %d", verb, code), func(t *testing.T) {
				t.Parallel()

				// Arrange
				fake := &fakeTask{replies: map[string]reply{verb: {err: exited(t, code, malformedLine)}}}

				// Act
				err := write(t.Context(), fake.client())

				// Assert
				if !errors.Is(err, taskwarrior.ErrRefused) {
					t.Errorf("%s returned %v, want ErrRefused", verb, err)
				}

				if seen := fmt.Sprint(err); strings.Contains(seen, taskrcSecret) {
					t.Errorf("%s repeated the taskrc secret: %s", verb, seen)
				}
			})
		}
	}
}

func TestDetectNeverRepeatsAMalformedTaskrcLine(t *testing.T) {
	t.Parallel()

	const malformed, good = "/malformed/task", "/good/task"

	tests := []struct {
		name       string
		program    string
		candidates []string
		replies    map[string]reply
		runs       int
	}{
		{
			name:    "its _version",
			program: taskProgram,
			replies: map[string]reply{versionWord: {err: exited(t, 2, malformedLine)}},
			runs:    1,
		},
		{
			name:    "its _show",
			program: taskProgram,
			replies: map[string]reply{versionWord: {stdout: versionAnswer}, showWord: {err: exited(t, 2, malformedLine)}},
			runs:    2,
		},
		{
			name:       "a _version ahead of a good Taskwarrior",
			candidates: []string{malformed, good},
			replies: map[string]reply{
				malformed + " _version": {err: exited(t, 2, malformedLine)},
				good + " _version":      {stdout: versionAnswer},
				showWord:                {stdout: ""},
			},
			runs: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := &fakeTask{replies: test.replies}

			// Act
			install, err := taskwarrior.Detect(t.Context(), test.program, test.candidates, fake.run)

			// Assert
			want := taskwarrior.ErrRefused.Error() + ": taskrc has a malformed line"
			if !errors.Is(err, taskwarrior.ErrRefused) || err.Error() != want {
				t.Errorf("Detect returned %v, want ErrRefused as %q", err, want)
			}

			if seen := fmt.Sprintf("%+v %v", install, err); strings.Contains(seen, taskrcSecret) {
				t.Errorf("Detect repeated the taskrc secret: %s", seen)
			}

			if len(fake.calls) != test.runs {
				t.Errorf("Detect ran %+v, want %d runs", fake.calls, test.runs)
			}
		})
	}
}
