// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// helperParent names, to a helper left behind, the pid of the helper that
// left it.
const helperParent = "WORKFLOW_PROC_PARENT"

// readyDescriptor is the descriptor a helper left behind says it has started
// on: the first of the helper's ExtraFiles, after standard input, output and
// error.
const readyDescriptor = 3

// actLeftBehind is the child in a mode about a process a program leaves
// behind holding its output: the program that leaves it, or the one left.
func actLeftBehind(mode string) {
	switch mode {
	case "background":
		// Leave a sleeping copy of this binary behind holding standard output, as
		// a hook that backgrounds a server does, print its pid, and exit at once.
		//nolint:noctx // this test binary re-run as the sleeper above, in a helper that has no context to thread
		grand := exec.Command(os.Args[0], "-test.run=^$")

		grand.Env = append(os.Environ(), helperMode+"=sleep")
		grand.Stdout = os.Stdout

		err := grand.Start()
		if err != nil {
			complain("starting the sleeper", err)

			return
		}

		fmt.Fprintln(os.Stdout, grand.Process.Pid)
	case "outlived":
		// Leave a copy of this binary behind holding standard output, which
		// writes there only once this one has exited, and exit once it says it
		// has started: how long a start takes is then no part of the wait the
		// run gives the pipe after the exit.
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
	case "late":
		// Say this one has started, then, once the program that left it behind
		// has exited and been reaped, print its pid and hold the output, as a
		// server a hook backgrounded does when it logs late.
		started := os.NewFile(readyDescriptor, "ready")
		_, err := started.Write([]byte{'\n'})
		complain("saying it started", err)
		complain("closing the pipe it said it started on", started.Close())

		awaitGone(os.Getenv(helperParent))
		fmt.Fprintln(os.Stdout, os.Getpid())
		time.Sleep(time.Minute)
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
