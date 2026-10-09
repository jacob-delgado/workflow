// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestRunStoppedByItsContextSaysTheInterfaceStopped(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Nothing ever arrives on the input, so only the context can end the run.
	never, _ := io.Pipe()

	t.Cleanup(func() { _ = never.Close() })

	// Act
	_, err := tui.Run(ctx, tui.New(config.Config{}, nil, tui.Deps{}), never, io.Discard)

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run under a canceled context = %v, want it to wrap %v", err, context.Canceled)
	}

	if err == nil || !strings.HasPrefix(err.Error(), "running the interface") {
		t.Errorf("Run under a canceled context = %v, want it to say it was running the interface", err)
	}
}

// runEnd is how a run of the interface ended: where it goes next, or why it
// failed.
type runEnd struct {
	next tui.Next
	err  error
}

// quitPatience bounds the wait for a run to end once q is typed, so an
// interface that never reads it fails the test rather than hanging it.
const quitPatience = 10 * time.Second

// typingPause is how long the run is left waiting before anything is typed:
// a run that ended on its own by then did not wait for its input.
const typingPause = 200 * time.Millisecond

func TestRunEndsWhenItsInputQuits(t *testing.T) {
	t.Parallel()

	// Arrange
	// The keys arrive on a pipe, so the run is seen waiting for them and
	// ending on the q, not because its input held nothing.
	keys, typing := io.Pipe()

	t.Cleanup(func() { _ = keys.Close() })

	ended := make(chan runEnd, 1)

	go func() {
		next, err := tui.Run(t.Context(), tui.New(config.Config{}, nil, tui.Deps{}), keys, io.Discard)
		ended <- runEnd{next: next, err: err}
	}()

	// Act: type nothing yet
	var early *runEnd

	select {
	case end := <-ended:
		early = &end
	case <-time.After(typingPause):
	}

	// Assert: the run waits for its input
	if early != nil {
		t.Fatalf("Run ended with %+v before anything was typed, want it waiting for a key", *early)
	}

	// Act: type q
	go func() { _, _ = typing.Write([]byte("q")) }()

	// Assert: the run ends, going nowhere
	select {
	case end := <-ended:
		if end.err != nil || end.next.Dir != "" {
			t.Errorf("Run reading q = %+v, want the interface to quit cleanly, going nowhere", end)
		}
	case <-time.After(quitPatience):
		t.Fatal("Run did not end once q was typed")
	}
}
