// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"errors"
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

// helperCue names, to a helper that waits on a cue, the file whose making
// cues it: a helper left behind to write, or the "closing" one to exit.
const helperCue = "WORKFLOW_PROC_CUE"

// helperPIDFile names, to a helper that leaves a process behind, the file to
// write that process's pid to, for the test to kill it however it ends.
const helperPIDFile = "WORKFLOW_PROC_PID_FILE"

// notePID writes pid to the file helperPIDFile names.
func notePID(pid int) {
	complain("noting the pid of the process left behind",
		os.WriteFile(os.Getenv(helperPIDFile), []byte(strconv.Itoa(pid)), 0o600))
}

// helperCuedLines names, to the "cued" helper, how many lines to write once
// cued: one when it is not set.
const helperCuedLines = "WORKFLOW_PROC_CUED_LINES"

// cuedBurst is how many lines a "cued" helper writes for a reader that is slow
// to take them: more than a run holds, so the rest wait in the pipe.
const cuedBurst = 150

// slowReader is how long a reader that is slow to come keeps the run waiting:
// far longer than any grace a run gives the output once the program exits.
const slowReader = time.Minute

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

// The statuses the "closing" helper exits with: closedStatus once cued, and
// neverCuedStatus if it gave up waiting for the cue.
const (
	closedStatus    = 3
	neverCuedStatus = 4
)

// stillOpenPatience is how long Lines is watched for closing while the program
// that ended its output goes on running.
const stillOpenPatience = 250 * time.Millisecond

// actAroundTheExit is the child in a mode about where a program's output ends
// against its exit: the program that leaves a process behind holding it past
// the exit, the one left, or a program ending it before.
func actAroundTheExit(mode string) {
	switch mode {
	case "closing":
		closeOutputThenExitOnCue()
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
// output, as a hook that backgrounds a server does, notes and prints its pid,
// and exits at once.
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

	notePID(grand.Process.Pid)
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

// writeLate says this one has started and notes its pid, then, once the
// program that left it behind has exited and been reaped, prints its pid and
// holds the output, as a server a hook backgrounded does when it logs late.
func writeLate() {
	started := os.NewFile(readyDescriptor, "ready")
	_, err := started.Write([]byte{'\n'})
	complain("saying it started", err)
	complain("closing the pipe it said it started on", started.Close())

	notePID(os.Getpid())
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

// writeOnceCued writes cuedLine once cued, as many times as helperCuedLines
// says, and exits, which ends the output.
func writeOnceCued() {
	awaitCue(os.Getenv(helperCue))

	count, err := strconv.Atoi(os.Getenv(helperCuedLines))
	if err != nil {
		count = 1
	}

	fmt.Fprint(os.Stdout, strings.Repeat(cuedLine+"\n", count))
}

// closeOutputThenExitOnCue closes standard output and error, which ends the
// run's output, then exits with closedStatus once cued, or with
// neverCuedStatus if the cue never came.
func closeOutputThenExitOnCue() {
	_ = os.Stdout.Close()
	_ = os.Stderr.Close()

	cue := os.Getenv(helperCue)
	awaitCue(cue)

	_, err := os.Stat(cue)
	if err != nil {
		os.Exit(neverCuedStatus)
	}

	os.Exit(closedStatus)
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

// Lines closes once the program has exited, as Output says, and not once
// its output has ended while it runs on.
func TestLinesStayOpenUntilAProgramThatEndedItsOutputExits(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	cue := filepath.Join(dir, "cue")

	output, err := proc.Start(t.Context(), child(dir, "closing", helperCue+"="+cue))
	if err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}

	// Act: read Lines while the program runs on with its output ended
	var (
		delivered, open bool
		line            string
	)

	select {
	case line, open = <-output.Lines:
		delivered = true
	case <-time.After(stillOpenPatience):
	}

	// Assert: Lines is still open, with nothing on it
	if delivered {
		t.Fatalf("Lines delivered %q (open %v) while the program ran, want it waiting on the exit", line, open)
	}

	// Act: cue the program to exit, and read the run to its end
	err = os.WriteFile(cue, nil, 0o600)
	if err != nil {
		t.Fatalf("cueing the program to exit: %v", err)
	}

	lines, err := collect(t, output)

	// Assert: Lines closes with nothing on it, and Wait reports the exit
	if len(lines) != 0 || !errors.Is(err, proc.ErrExitStatus) ||
		!strings.Contains(err.Error(), "exit status "+strconv.Itoa(closedStatus)) {
		t.Errorf("the run ended with %q and %v, want no lines and exit status %d", lines, err, closedStatus)
	}
}

// The grace a run gives a process left behind holding its output counts only
// while the run waits on the pipe, never while its lines wait on a reader: a
// reader slow past any grace, once the program has exited, still gets every
// line, whether it is slow while the program's last lines wait or while the
// process left behind writes more than the run holds.
func TestAReaderSlowPastTheGraceStillGetsEveryLine(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		// As in TestALineWrittenOnceTheRunHasTakenTheExitStillArrives, the run
		// has taken the exit once synctest.Wait returns, its lines waiting on a
		// reader. The bubble's clock moves only while every goroutine of the run
		// waits on another, never while it waits on the pipe.
		dir := t.TempDir()
		cue := filepath.Join(dir, "cue")

		output, err := proc.Start(t.Context(), child(dir, "unread",
			helperCue+"="+cue, helperCuedLines+"="+strconv.Itoa(cuedBurst)))
		if err != nil {
			t.Fatalf("Start returned %v, want nil", err)
		}

		synctest.Wait()

		// Act: stay slow while the program's last lines wait, then read them
		time.Sleep(slowReader)

		unread := make([]string, 0, unreadLines)
		for line := range output.Lines {
			unread = append(unread, line)
			if len(unread) == unreadLines {
				break
			}
		}

		// Assert: every one of them came
		if !slices.Equal(unread, linesWrittenUnread()) {
			t.Fatalf("a reader slow once the program exited read %d of its %d lines before Lines closed",
				len(unread), unreadLines)
		}

		// Act: cue the process left behind, stay slow while its lines wait, then
		// read the run to its end
		err = os.WriteFile(cue, nil, 0o600)
		if err != nil {
			t.Fatalf("cueing the process left behind: %v", err)
		}

		synctest.Wait()
		time.Sleep(slowReader)

		cued, err := collect(t, output)

		// Assert: every line it wrote came, and the run ended well
		if err != nil || len(cued) != cuedBurst || slices.ContainsFunc(cued, func(line string) bool {
			return line != cuedLine
		}) {
			t.Errorf("a reader slow while the process left behind wrote got %d lines and %v, want %d %q lines and "+
				"no failure", len(cued), err, cuedBurst, cuedLine)
		}
	})
}
