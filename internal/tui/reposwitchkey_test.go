// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"
)

// switchDirKey is the key the Repositories pane switches to the row under the
// cursor with, by default.
const switchDirKey = "s"

// Apply confirms a switch's last look, so on a row it switches nothing: a
// double press of it would otherwise ask and then switch, dropping whatever
// the look lists as lost.
func TestEnterOnARowSwitchesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	onRow := onWeb(t, reposWorld())

	// Act
	stayed, cmd := pressed(t, onRow, keyEnter)

	// Assert
	if quits(cmd) || stayed.Destination().Dir != "" {
		t.Errorf("enter on a row left for %q", stayed.Destination().Dir)
	}

	if context, open := stayed.OverlayKeyContext(); open {
		t.Errorf("enter on a row opened %s, want nothing opened", context.Name)
	}
}

func TestTheSwitchKeyAsksAndApplySwitches(t *testing.T) {
	t.Parallel()

	// Arrange
	onRow := onWeb(t, reposWorld())

	// Act: press the switch key on the web row
	asked, cmd := pressed(t, onRow, switchDirKey)

	// Assert: the switch's last look opens, and nothing is left yet
	if quits(cmd) || asked.Destination().Dir != "" {
		t.Fatalf("left for %q at once, want a last look first", asked.Destination().Dir)
	}

	requireScreen(t, asked.View().Content, "Switch to ~/src/web?", "enter switch", "esc stay")

	// Act: confirm the look
	left, cmd := pressed(t, asked, keyEnter)

	// Assert: the interface ends for the web directory
	if !quits(cmd) || left.Destination().Dir != webRoot {
		t.Errorf("enter on the look: quit %v, destination %q; want the program ended for %s",
			quits(cmd), left.Destination().Dir, webRoot)
	}
}
