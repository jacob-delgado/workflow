// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package tui_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// pipedKeysMode is the environment variable that turns this test binary into
// the child TestRunReadsNoKeysFromAFileThatIsNoTerminal starts: re-running the
// test binary is how a test gets a process with no terminal at all, which this
// one, run from a developer's shell, may well have.
const pipedKeysMode = "WORKFLOW_TUI_PIPED_KEYS"

// childPatience bounds the child's run, so an interface that waits for keys
// it will never get fails the test rather than hanging it.
const childPatience = 10 * time.Second

// TestMain makes this test binary the child a test starts when pipedKeysMode
// is set, and otherwise runs the tests. The child is dispatched here rather
// than from a test function, so it never shows up as a test that asserts
// nothing.
func TestMain(m *testing.M) {
	if os.Getenv(pipedKeysMode) != "" {
		// Exits by itself: gobco rewrites an os.Exit written in TestMain to save
		// its counters, which the child would otherwise race to overwrite.
		runOverAPipedQ()
	}

	os.Exit(m.Run())
}

// runOverAPipedQ is the child: it prints what of a pipe holding a q the
// interface left unread, and exits.
func runOverAPipedQ() {
	left, err := keysLeftOnAPipe("q")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, left)
	os.Exit(0)
}

// keysLeftOnAPipe runs the interface over a pipe holding typed, and returns
// what of it the run left unread.
func keysLeftOnAPipe(typed string) (string, error) {
	keys, typing, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("making the pipe: %w", err)
	}
	defer func() { _ = keys.Close() }()

	_, err = typing.WriteString(typed)
	if err == nil {
		err = typing.Close()
	}

	if err != nil {
		return "", fmt.Errorf("typing on the pipe: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), childPatience)
	defer cancel()

	// Whether the run fails or quits is not the point: where it read keys is.
	_, _ = tui.Run(ctx, tui.New(config.Config{}, nil, tui.Deps{}), keys, io.Discard)

	left, err := io.ReadAll(keys)
	if err != nil {
		return "", fmt.Errorf("reading what is left on the pipe: %w", err)
	}

	return string(left), nil
}

func TestRunReadsNoKeysFromAFileThatIsNoTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	// The pipe or /dev/null standard input may be holds no keys; the terminal
	// the process runs on does, so that is where Bubble Tea reads them from.
	// A session of its own gives the child no terminal to open, so its run
	// ends there at once rather than taking over the developer's.
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$")

	child.Env = append(os.Environ(), pipedKeysMode+"=1")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Act
	left, err := child.Output()
	// Assert
	if err != nil {
		t.Fatalf("the child running the interface over a pipe failed: %v", err)
	}

	if string(left) != "q" {
		t.Errorf("Run left %q of a pipe's keys unread, want %q: a file that is no terminal gives no keys", left, "q")
	}
}
