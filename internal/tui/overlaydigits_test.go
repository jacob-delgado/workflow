// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// letterTyped is a letter no action is moved onto, pressed to learn whether an
// overlay types what it is handed into a field.
const letterTyped = "z"

// paneNumbers are the keys jump-to-pane answers, the one action the probe
// never moves.
func paneNumbers(t *testing.T) []string {
	t.Helper()

	for _, action := range tui.KeyActions("pull request", "Slack", nil) {
		if action.Action == fixedAction {
			return action.Keys
		}
	}

	t.Fatalf("the help lists no %s", fixedAction)

	return nil
}

// digitProbe is what pressing the pane numbers in one state of an overlay
// finds: whether the state types a letter into a field, and each pane number
// a press of answers.
type digitProbe struct {
	overlay  string
	state    int
	types    bool
	answered []string
}

// probeDigits opens every overlay in each of its states, with every action
// moved off the keys it has by default, and presses a letter and each pane
// number on it.
func probeDigits(t *testing.T, states map[string][]overlayState, digits []string) []digitProbe {
	t.Helper()

	moved := movedKeys(t)

	var probes []digitProbe

	for _, overlay := range slices.Sorted(maps.Keys(states)) {
		for index, state := range states[overlay] {
			model, _ := openedQuiet(t, state, moved)
			unanswered, _ := model.Update(noKey())
			probe := digitProbe{overlay: overlay, state: index, types: answers(model, keyMsg(letterTyped), unanswered)}

			for _, digit := range digits {
				if answers(model, keyMsg(digit), unanswered) {
					probe.answered = append(probe.answered, digit)
				}
			}

			probes = append(probes, probe)
		}
	}

	return probes
}

// An overlay has the keyboard: a pane number it does not answer must not
// reach the panes behind it, switching to another while the overlay stays
// open, nor change the overlay. That is what lets CheckKeys put an overlay's
// own action, such as a running command's stop, on a pane number. An overlay
// with a text field types a digit as it types a letter, so the states that
// type nothing are the ones held to it.
func TestAPaneNumberChangesNothingInAnOverlayThatTypesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	digits := paneNumbers(t)

	// Act
	probes := probeDigits(t, overlayStates(), digits)

	// Assert
	quiet := slices.DeleteFunc(slices.Clone(probes), func(probe digitProbe) bool { return probe.types })
	if len(quiet) == 0 {
		t.Fatalf("every one of the %d overlay states probed types %q into a field: no state is held to the pane "+
			"numbers", len(probes), letterTyped)
	}

	for _, probe := range quiet {
		if len(probe.answered) > 0 {
			t.Errorf("state %d of %s answers the pane numbers %q, though it types nothing", probe.state,
				probe.overlay, probe.answered)
		}
	}
}
