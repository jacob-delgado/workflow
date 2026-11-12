// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package proc_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// errCallerGone is the cause a test's caller ends its context with.
var errCallerGone = errors.New("the caller went away")

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

// A context that ended before the program could start reports its cause too,
// though no kill reached a program: exec's Start answers with the context's
// error alone.
func TestCaptureAnswersAContextEndedBeforeTheStartWithItsCause(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(errCallerGone)

	// Act
	_, err := proc.Capture(ctx, script("exit 0"), nil)

	// Assert
	if !errors.Is(err, errCallerGone) {
		t.Errorf("Capture under a context ended before the start returned %v, want its cause %v", err, errCallerGone)
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

// A program that exits with a status of its own reports that status, though
// the context ends before the run returns: the context is canceled only once
// the program has been reaped, while a process it left behind still holds its
// output open, so the exit came first every run.
func TestCaptureReportsAnExitThatCameBeforeTheContextEnded(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	reaped, released := filepath.Join(dir, "reaped"), filepath.Join(dir, "released")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		awaitFile(t, reaped)
		cancel()

		err := os.WriteFile(released, nil, 0o600)
		if err != nil {
			t.Errorf("releasing the process left behind: %v", err)
		}
	}()

	// Act
	_, err := proc.Capture(ctx, script(`(while kill -0 $$ 2>/dev/null; do sleep 0.01; done; : > '`+reaped+`'; `+
		`while [ ! -e '`+released+`' ] && [ -d '`+dir+`' ]; do sleep 0.01; done) & exit 3`), nil)

	// Assert
	if code, _, exited := proc.Failure(err); !exited || code != 3 {
		t.Errorf("Capture of a program that exited 3 before its context ended returned %v, want exit status 3", err)
	}
}

// awaitFile waits until path is there, for a minute at most.
func awaitFile(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(time.Minute)

	for {
		_, err := os.Stat(path)
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			t.Errorf("waiting for %s: %v", path, err)

			return
		}

		time.Sleep(time.Millisecond)
	}
}
