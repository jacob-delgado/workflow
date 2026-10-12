// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// lastLookContext is the key context a last look opens in.
const lastLookContext = "a last look"

// An action whose press opens a last look, on the key apply has, would have
// that key pressed twice ask and then confirm. The probes find every such
// action in every pane and overlay state, so one added later is held to it
// too, wherever it is.

// requireRefusedOnApply fails for each action CheckKeys accepts on apply's
// key, enter.
func requireRefusedOnApply(t *testing.T, asking []string) {
	t.Helper()

	for _, action := range asking {
		err := tui.CheckKeys(map[string]string{action: keyEnter})
		if err == nil {
			t.Errorf("%s opens a last look, yet CheckKeys accepts it on %s, the key that confirms the look", action,
				keyEnter)
		}
	}
}

func TestAPaneActionThatOpensALastLookIsRefusedOnTheKeyThatConfirmsIt(t *testing.T) {
	t.Parallel()

	for name, state := range paneStates() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			asking := sweepAt(t, state).asks

			// Assert
			requireRefusedOnApply(t, asking)
		})
	}
}

func TestAnOverlayActionThatOpensALastLookIsRefusedOnTheKeyThatConfirmsIt(t *testing.T) {
	t.Parallel()

	for name, states := range overlayStates() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			probes := probeStates(t, states)

			// Assert
			for _, probe := range probes {
				requireRefusedOnApply(t, probe.asks)
			}
		})
	}
}

// The probes are only as good as their sense of a last look opening: push on
// an unpushed branch opens one, and so does removing the cache in Local data.
func TestTheProbesSeeALastLookOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	branch, localData := paneStates()["Branch, unpushed"], overlayStates()["Local data"][0]

	// Act
	push, removals := probeAt(t, branch, pushAction), probeOverlay(t, localData)

	// Assert
	if !push.asks || !slices.Contains(removals.asks, "remove-cache") {
		t.Errorf("push opens a last look: %v; Local data's actions that open one: %q, want remove-cache among them",
			push.asks, removals.asks)
	}
}
