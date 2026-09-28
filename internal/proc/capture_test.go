// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// script describes a run of sh with one command line.
func script(line string) proc.Command {
	return proc.Command{Name: "sh", Args: []string{"-c", line}}
}

func TestCaptureWithinStopsAProgramAtItsBound(t *testing.T) {
	t.Parallel()

	// Arrange
	start := time.Now()

	// Act
	_, err := proc.CaptureWithin(t.Context(), 50*time.Millisecond, script("exec sleep 5"), nil)

	// Assert
	if !errors.Is(err, proc.ErrTimedOut) || !strings.Contains(err.Error(), "50ms") {
		t.Errorf("CaptureWithin returned %v, want ErrTimedOut naming its 50ms bound", err)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("CaptureWithin took %s, want it stopped near its 50ms bound", elapsed)
	}
}

func TestCaptureWithinReturnsTheOutputWrittenBeforeItsBound(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.CaptureWithin(t.Context(), 100*time.Millisecond, script("echo partial; exec sleep 5"), nil)

	// Assert
	if !errors.Is(err, proc.ErrTimedOut) {
		t.Errorf("CaptureWithin returned %v, want ErrTimedOut", err)
	}

	if got := strings.TrimSpace(string(output)); got != "partial" {
		t.Errorf("CaptureWithin output = %q, want %q", got, "partial")
	}
}

func TestCaptureWithinLeavesATighterParentDeadlineUnclaimed(t *testing.T) {
	t.Parallel()

	// Arrange
	parent, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	// Act
	// The caller's deadline ends the run long before CaptureWithin's own minute.
	_, err := proc.CaptureWithin(parent, time.Minute, script("exec sleep 5"), nil)

	// Assert
	if errors.Is(err, proc.ErrTimedOut) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("CaptureWithin under a caller's tighter deadline returned %v, "+
			"want the caller's context.DeadlineExceeded, not its own timeout", err)
	}
}

func TestCaptureWithinAnswersACallersCancelAsCanceled(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	time.AfterFunc(50*time.Millisecond, cancel)

	// Act
	_, err := proc.CaptureWithin(ctx, time.Minute, script("exec sleep 5"), nil)

	// Assert
	if errors.Is(err, proc.ErrTimedOut) || !errors.Is(err, context.Canceled) {
		t.Errorf("CaptureWithin canceled by its caller mid-run returned %v, "+
			"want context.Canceled, not its own timeout", err)
	}

	if want := "sh: context canceled"; err == nil || err.Error() != want {
		t.Errorf("CaptureWithin canceled by its caller mid-run returned %v, want %q", err, want)
	}
}

func TestCaptureWithinKeepsTheOutputOfAFailedProgram(t *testing.T) {
	t.Parallel()

	// Act
	output, err := proc.CaptureWithin(t.Context(), time.Minute, script("echo '[]'; exit 2"), nil)

	// Assert
	if err == nil {
		t.Error("CaptureWithin of a program exiting 2 returned no error")
	}

	if got := strings.TrimSpace(string(output)); got != "[]" {
		t.Errorf("CaptureWithin output = %q, want %q", got, "[]")
	}
}
