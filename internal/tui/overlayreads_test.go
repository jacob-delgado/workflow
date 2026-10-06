// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
)

// readingConfiguration is what Settings says while its read is under way.
const readingConfiguration = "reading the configuration"

func TestSettingsWhileReadingTakesOnlyEsc(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	reading, _ := pressed(t, typing(t, repo.live(t, 120, 40), reposKey), settingsKey)

	// Act: look while the read is under way
	view := reading.View().Content

	// Assert: it says so, and offers only to close
	requireScreen(t, view, readingConfiguration)

	if footer := footerLine(view); !strings.Contains(footer, "esc close") || strings.Contains(footer, saveKey) {
		t.Errorf("footer %q, want only esc close while reading", footer)
	}

	// Act: try to save before anything was read
	still, cmd := pressed(t, reading, saveKey)

	// Assert: nothing is saved, and it is still reading
	if cmd != nil || len(repo.asked("save-settings")) != 0 {
		t.Errorf("saving while reading ran %v, saved %d times; want nothing", cmd != nil, len(repo.asked("save-settings")))
	}

	requireScreen(t, still.View().Content, readingConfiguration)
}

func TestASettingsReadArrivingAfterSettingsClosedIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	reading, read := pressed(t, typing(t, newWorld().live(t, 120, 40), reposKey), settingsKey)
	closed := typing(t, reading, keyEsc)

	// Act
	arrived, _ := finish(t, closed, read)

	// Assert
	refuseScreen(t, arrived.View().Content, "Saves to")
}

func TestASettingsReadArrivingForAnEarlierOpeningIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	model := typing(t, newWorld().live(t, 120, 40), reposKey)
	first, firstRead := pressed(t, model, settingsKey)
	reopened, _ := pressed(t, typing(t, first, keyEsc), settingsKey)

	// Act
	arrived, _ := finish(t, reopened, firstRead)

	// Assert
	requireScreen(t, arrived.View().Content, readingConfiguration)
}

func TestSettingsWhileSavingTakesNoKey(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	saving, _ := pressed(t, typing(t, repo.live(t, 120, 40), edited(projectRow, editedProject)...), saveKey)

	// Act
	after, cmd := pressed(t, saving, keyEsc)

	// Assert
	if cmd != nil {
		t.Error("esc while saving ran a command, want it ignored")
	}

	view := after.View().Content
	requireScreen(t, view, "Saves to")

	if footer := footerLine(view); !strings.Contains(footer, "ctrl+c") || strings.Contains(footer, "esc") {
		t.Errorf("footer %q, want only ctrl+c while saving", footer)
	}
}

func TestLocalDataWhileReadingSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	reading, _ := pressed(t, typing(t, newWorld().live(t, 120, 40), reposKey), localDataKey)

	// Act
	view := reading.View().Content

	// Assert
	requireScreen(t, view, "reading the local data")
	refuseScreen(t, view, "Kept in")
}

func TestALocalDataReadArrivingAfterItClosedIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	reading, read := pressed(t, typing(t, newWorld().live(t, 120, 40), reposKey), localDataKey)
	closed := typing(t, reading, keyEsc)

	// Act
	arrived, _ := finish(t, closed, read)

	// Assert
	refuseScreen(t, arrived.View().Content, "Kept in")
}

func TestALocalDataReadArrivingForAnEarlierOpeningIsDropped(t *testing.T) {
	t.Parallel()

	// Arrange
	model := typing(t, newWorld().live(t, 120, 40), reposKey)
	first, firstRead := pressed(t, model, localDataKey)
	reopened, _ := pressed(t, typing(t, first, keyEsc), localDataKey)

	// Act
	arrived, _ := finish(t, reopened, firstRead)

	// Assert
	requireScreen(t, arrived.View().Content, "reading the local data")
}
