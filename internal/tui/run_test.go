// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

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

func TestRunEndsWhenItsInputQuits(t *testing.T) {
	t.Parallel()

	// Arrange
	model := tui.New(config.Config{}, nil, tui.Deps{})

	// Act
	next, err := tui.Run(t.Context(), model, strings.NewReader("q"), io.Discard)
	// Assert
	if err != nil {
		t.Fatalf("Run reading q = %v, want the interface to quit cleanly", err)
	}

	if next.Dir != "" {
		t.Errorf("Run reading q goes to %q next, want it to go nowhere", next.Dir)
	}
}
