// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package proc_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

func TestStartKillsTheGrandchildWhenTheRunIsCanceled(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	output, err := proc.Start(ctx, child(t.TempDir(), "grandchild"))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	grandchild := grandchildPID(t, output)

	// Act
	// Cancel the run, as a stop or a quit would.
	cancel()

	_, _ = collect(t, output)

	// Assert
	// The grandchild does not outlive the canceled run, where the child alone
	// being killed would leave it running with a parent of init.
	if !gone(grandchild) {
		t.Errorf("grandchild %d survived the cancel", grandchild)
	}
}

func TestARunEndsThoughAProcessItLeftBehindHoldsItsOutput(t *testing.T) {
	t.Parallel()

	// Each helper exits at once, leaving a process behind that holds the pipe
	// open for a minute and prints its pid, as a server a hook backgrounds
	// does: at once, or only once the helper is gone, when the wait on the
	// pipe pauses while that line is handed on and counts on once it has been.
	cases := map[string]string{
		"a process that writes before the program exits": "background",
		"a process that writes after the program exits":  "outlived",
	}

	for name, mode := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			output, err := proc.Start(t.Context(), child(t.TempDir(), mode))
			if err != nil {
				t.Fatalf("Start returned %v, want nil", err)
			}

			type ending struct {
				lines []string
				err   error
			}

			ended := make(chan ending, 1)

			// Act
			go func() {
				lines, err := collect(t, output)
				ended <- ending{lines: lines, err: err}
			}()

			// Assert
			select {
			case end := <-ended:
				t.Cleanup(func() { killAll(end.lines) })

				if end.err != nil || len(end.lines) != 1 {
					t.Errorf("the run ended with %q and %v, want the pid it printed and no failure", end.lines, end.err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Lines stayed open and Wait blocked while the process left behind held the output")
			}
		})
	}
}

// killAll kills each process a line names by its pid.
func killAll(lines []string) {
	for _, line := range lines {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// grandchildPID reads the pid the helper printed for the process it spawned.
func grandchildPID(t *testing.T, output proc.Output) int {
	t.Helper()

	for line := range output.Lines {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil {
			return pid
		}
	}

	t.Fatal("the grandchild helper printed no pid")

	return 0
}

// gone reports whether pid names no live process, polling because the kill and
// the reaping that follows it are not instant.
func gone(pid int) bool {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}

		time.Sleep(10 * time.Millisecond)
	}

	return false
}
