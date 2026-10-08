// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package pgroup_test

import (
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/jacob-delgado/workflow/internal/proc/pgroup"
)

// isolated is a command for program, isolated in a group of its own.
func isolated(t *testing.T, program string, args ...string) *exec.Cmd {
	t.Helper()

	command := exec.CommandContext(t.Context(), program, args...)
	pgroup.Isolate(command)

	return command
}

func TestCancelBeforeTheCommandStartsHasNothingToKill(t *testing.T) {
	t.Parallel()

	// Arrange
	command := isolated(t, "sleep", "60")

	// Act
	err := command.Cancel()
	// Assert
	if err != nil {
		t.Errorf("Cancel before Start = %v, want nil", err)
	}
}

func TestCancelKillsTheCommandsGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	command := isolated(t, "sleep", "60")

	err := command.Start()
	if err != nil {
		t.Fatalf("starting sleep: %v", err)
	}

	// Act
	err = command.Cancel()

	// Assert
	waited := command.Wait()

	exit, exited := errors.AsType[*exec.ExitError](waited)
	if err != nil || !exited || exit.String() != "signal: killed" {
		t.Errorf("Cancel = %v, then Wait = %v; want the group killed", err, waited)
	}
}

func TestCancelOnceTheGroupIsGoneFindsTheProcessDone(t *testing.T) {
	t.Parallel()

	// Arrange
	command := isolated(t, "true")

	err := command.Run()
	if err != nil {
		t.Fatalf("running true: %v", err)
	}

	// Act
	err = command.Cancel()

	// Assert
	if !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("Cancel once the group is gone = %v, want os.ErrProcessDone", err)
	}
}
