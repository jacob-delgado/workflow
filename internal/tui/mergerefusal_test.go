// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestAMoveToAnotherMethodKeepsTheForgesRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	// A forge refuses the pull request more often than the method, so the
	// refusal stays until another merge is sent.
	reviewing := mergeableTwoWays()
	reviewing.mergeErr = forge.ErrRefused
	refused := typing(t, reviewing.live(t, 120, 40), "4", "M", keyEnter)

	// Act
	moved := typing(t, refused, "j")

	// Assert
	requireScreen(t, moved.View().Content, "▸ squash and merge", "✗ The forge refused this", "enter merge")
}

func TestAMergeSentAgainDropsTheRefusalWhileItIsOut(t *testing.T) {
	t.Parallel()

	// Arrange
	reviewing := mergeableTwoWays()
	reviewing.mergeErr = forge.ErrRefused
	refused := typing(t, reviewing.live(t, 120, 40), "4", "M", keyEnter)

	// Act
	resent, _ := pressed(t, refused, keyEnter)

	// Assert
	requireScreen(t, resent.View().Content, "merging…", "ctrl+c quit")
	refuseScreen(t, resent.View().Content, "✗ The forge refused this")
}
