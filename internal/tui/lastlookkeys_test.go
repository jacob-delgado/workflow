// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// lastLookContext is the key context a last look opens in.
const lastLookContext = "a last look"

// An action whose press opens a last look, on the key apply has, would have
// that key pressed twice ask and then confirm. The probes find every such
// action in every pane and overlay state, so one added later is held to it
// too, wherever it is.

// requireRefusedOnApply fails for each action CheckKeys does not refuse on
// apply's key, enter, as one that asks first.
func requireRefusedOnApply(t *testing.T, asking []string) {
	t.Helper()

	for _, action := range asking {
		err := tui.CheckKeys(map[string]string{action: keyEnter})
		if !errors.Is(err, config.ErrKeyConfirmsItself) {
			t.Errorf("%s opens a last look, yet CheckKeys answers %v on %s, the key that confirms the look; "+
				"want it refused as asking first", action, err, keyEnter)
		}
	}
}

// refusedAsAskingFirst is every action CheckKeys refuses on apply's key,
// enter, as one whose press opens a last look.
func refusedAsAskingFirst() []string {
	var refused []string

	for _, action := range tui.KeyActions("pull request", "Slack", nil) {
		err := tui.CheckKeys(map[string]string{action.Action: keyEnter})
		if errors.Is(err, config.ErrKeyConfirmsItself) {
			refused = append(refused, action.Action)
		}
	}

	return refused
}

// asksSomewhere reports that a press of action opens a last look in a pane
// state that answers it, or in a state of an overlay whose key context lists
// it.
func asksSomewhere(t *testing.T, action string) bool {
	t.Helper()

	for _, state := range paneStates() {
		if slices.Contains(strings.Fields(state.answers), action) && probeAt(t, state, action).asks {
			return true
		}
	}

	for _, context := range tui.OverlayKeyContexts() {
		if !slices.Contains(context.Actions, action) {
			continue
		}

		for _, probe := range probeStates(t, overlayStates()[context.Name]) {
			if slices.Contains(probe.asks, action) {
				return true
			}
		}
	}

	return false
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

// The probes hold every action that opens a last look to the refusal; this
// holds the refusal to the probes, so an action kept off apply's key that
// opens none is not refused for nothing.
func TestAnActionRefusedAsAskingFirstOpensALastLookWhereTheProbesLook(t *testing.T) {
	t.Parallel()

	// Act
	refused := refusedAsAskingFirst()

	// Assert
	if len(refused) == 0 {
		t.Fatal("CheckKeys refuses no action on enter as asking first")
	}

	for _, action := range refused {
		if !asksSomewhere(t, action) {
			t.Errorf("CheckKeys refuses %s on %s as asking first, yet no probed state sees it open a last look",
				action, keyEnter)
		}
	}
}
