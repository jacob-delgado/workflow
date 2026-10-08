// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package pgroup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// killGrace is how long Wait gives the child to exit once the group is
// signaled before it kills the child itself. It closes no output: exec closes
// only the pipes it made, and a streamed run hands the child a pipe of its own,
// so a process still holding the output once the child is gone is for the
// run's reader to stop waiting on (proc's outputGrace).
const killGrace = 2 * time.Second

// Isolate puts command in its own process group and, when its context is
// canceled, kills the whole group rather than the child alone.
//
// The default cancel kills only the child, so a grandchild it spawned — the ssh
// or gpg a push runs to ask for a credential — is reparented to init and lives
// on, holding the terminal the interface owns and finishing work the user meant
// to stop. Signaling the negative group id reaches the child and everything it
// started.
func Isolate(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = killGrace
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}

		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			// The group is already gone; that is the outcome cancel wanted.
			return os.ErrProcessDone
		}

		if err != nil {
			return fmt.Errorf("killing the process group: %w", err)
		}

		return nil
	}
}
