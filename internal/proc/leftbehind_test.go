// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// helperParent names, to a helper left behind, the pid of the helper that
// left it.
const helperParent = "WORKFLOW_PROC_PARENT"

// readyDescriptor is the descriptor a helper left behind says it has started
// on: the first of the helper's ExtraFiles, after standard input, output and
// error.
const readyDescriptor = 3

// helperCue names, to a helper left behind, the file whose making cues it to
// write.
const helperCue = "WORKFLOW_PROC_CUE"

// unreadLines is how many lines the "unread" helper writes before it exits:
// more than a run holds for a reader that has yet to come, so the last of them
// waits for one, and few enough to wait in the pipe meanwhile, so the program
// still exits. What a run holds is start.go's unexported lineBuffer, and this
// count must stay above it: a run holding every line would wait on the
// pipe, which a bubble's Wait does not count as blocked, until the helper left
// behind gave up its minute's wait for the cue and wrote that it did, failing
// the test a minute late with that complaint among the lines.
const unreadLines = 200

// cuedLine is what the "cued" helper writes once cued.
const cuedLine = "cued"

// actLeftBehind is the child in a mode about a process a program leaves
// behind holding its output: the program that leaves it, or the one left.
func actLeftBehind(mode string) {
	switch mode {
	case "background":
		leaveASleeper()
	case "outlived":
		leaveALateWriter()
	case "late":
		writeLate()
	case "unread":
		leaveACuedWriter()
	case "cued":
		writeOnceCued()
	}
}

// leaveASleeper leaves a sleeping copy of this binary behind holding standard
// output, as a hook that backgrounds a server does, prints its pid, and exits
// at once.
func leaveASleeper() {
	//nolint:noctx // this test binary re-run as the "sleep" helper, in a helper that has no context to thread
	grand := exec.Command(os.Args[0], "-test.run=^$")

	grand.Env = append(os.Environ(), helperMode+"=sleep")
	grand.Stdout = os.Stdout

	err := grand.Start()
	if err != nil {
		complain("starting the sleeper", err)

		return
	}

	fmt.Fprintln(os.Stdout, grand.Process.Pid)
}

// leaveALateWriter leaves a copy of this binary behind holding standard
// output, which writes there only once this one has exited, and exits once it
// says it has started: how long a start takes is then no part of the wait the
// run gives the pipe after the exit.
func leaveALateWriter() {
	ready, readyWriter, err := os.Pipe()
	if err != nil {
		complain("opening the pipe the late writer says it started on", err)

		return
	}

	//nolint:noctx // this test binary re-run as the late writer below, in a helper that has no context to thread
	late := exec.Command(os.Args[0], "-test.run=^$")

	late.Env = append(os.Environ(), helperMode+"=late", helperParent+"="+strconv.Itoa(os.Getpid()))
	late.Stdout, late.Stderr = os.Stdout, os.Stderr
	late.ExtraFiles = []*os.File{readyWriter}

	complain("starting the late writer", late.Start())
	complain("closing this end of the pipe it says it started on", readyWriter.Close())

	_, err = ready.Read(make([]byte, 1))
	complain("waiting for the late writer to say it started", err)
}

// writeLate says this one has started, then, once the program that left it
// behind has exited and been reaped, prints its pid and holds the output, as a
// server a hook backgrounded does when it logs late.
func writeLate() {
	started := os.NewFile(readyDescriptor, "ready")
	_, err := started.Write([]byte{'\n'})
	complain("saying it started", err)
	complain("closing the pipe it said it started on", started.Close())

	awaitGone(os.Getenv(helperParent))
	fmt.Fprintln(os.Stdout, os.Getpid())
	time.Sleep(time.Minute)
}

// leaveACuedWriter writes more than a run holds for a reader that has yet to
// come, leaves behind a copy of this binary holding standard output, which
// writes one line more only once cued, and exits at once.
func leaveACuedWriter() {
	fmt.Fprint(os.Stdout, strings.Join(linesWrittenUnread(), "\n")+"\n")

	//nolint:noctx // this test binary re-run as the cued writer below, in a helper that has no context to thread
	cued := exec.Command(os.Args[0], "-test.run=^$")

	cued.Env = append(os.Environ(), helperMode+"=cued")
	cued.Stdout, cued.Stderr = os.Stdout, os.Stderr

	complain("starting the cued writer", cued.Start())
}

// writeOnceCued writes one line once cued and exits, which ends the output.
func writeOnceCued() {
	awaitCue(os.Getenv(helperCue))
	fmt.Fprintln(os.Stdout, cuedLine)
}

// linesWrittenUnread are the lines the "unread" helper writes before it
// exits, in order.
func linesWrittenUnread() []string {
	lines := make([]string, unreadLines)
	for i := range lines {
		lines[i] = strconv.Itoa(i)
	}

	return lines
}

// awaitCue waits until the file path names is there, for a minute at most, so
// a test that never cues does not leave the helper behind for good.
func awaitCue(path string) {
	deadline := time.Now().Add(time.Minute)

	for {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			complain("waiting to be cued", err)

			return
		}

		time.Sleep(time.Millisecond)
	}
}

// awaitGone waits until the process pid names has exited and been reaped.
func awaitGone(pid string) {
	parent, err := strconv.Atoi(pid)
	if err != nil {
		complain("reading the pid of the program that left it", err)

		return
	}

	process, err := os.FindProcess(parent)
	if err != nil {
		complain("finding the program that left it", err)

		return
	}

	for process.Signal(syscall.Signal(0)) == nil {
		time.Sleep(time.Millisecond)
	}
}

func TestALineWrittenOnceTheRunHasTakenTheExitStillArrives(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// The helper writes more than the run holds for a reader that has yet to
		// come, leaves behind a process holding the output that writes one line
		// more only once cued, and exits. synctest.Wait returns once every
		// goroutine of the run waits on another of them, and none does while the
		// wait for the program is in its system call or the run has yet to take
		// the exit it returned: so the cued line comes after the exit, every run.
		// The bubble's clock stands still while the run waits on the pipe, so the
		// grace the run gives that process cannot run out here:
		// TestARunEndsThoughAProcessItLeftBehindHoldsItsOutput holds that on a
		// real clock.
		dir := t.TempDir()
		cue := filepath.Join(dir, "cue")

		output, err := proc.Start(t.Context(), child(dir, "unread", helperCue+"="+cue))
		if err != nil {
			t.Fatalf("Start returned %v, want nil", err)
		}

		synctest.Wait()

		// Act
		err = os.WriteFile(cue, nil, 0o600)
		if err != nil {
			t.Fatalf("cueing the process left behind: %v", err)
		}

		lines, err := collect(t, output)

		// Assert
		want := append(linesWrittenUnread(), cuedLine)
		if err != nil || !slices.Equal(lines, want) {
			t.Errorf("the run ended with %d lines, the last two %q, and %v; want %d, the last two %q, and no failure",
				len(lines), lines[max(0, len(lines)-2):], err, len(want), want[len(want)-2:])
		}
	})
}
