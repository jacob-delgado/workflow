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
