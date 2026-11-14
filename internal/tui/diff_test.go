// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// errDiffRead is git failing to read a diff.
var errDiffRead = errors.New("could not read the diff")

// redactPath is the file the default world has changed.
const redactPath = "internal/config/redact.go"

func TestTheSelectedFilesDiffShowsWithItsMarks(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.diff = []string{"@@ -1,2 +1,2 @@", "-old line", "+new line", " context"}

	// Act
	view := typing(t, world.live(t, 120, 40), "3").View().Content

	// Assert
	// The added and removed lines keep git's + and -, so they read without color.
	requireScreen(t, view, "diff: "+redactPath, "-old line", "+new line")
}

func TestADiffWithNoTextualChangeSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// A mode-only change reads as no lines.
	world := newWorld()
	world.diff = nil

	// Act
	view := typing(t, world.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view, "no textual change")
}

func TestMovingTheSelectionReadsTheNewFilesDiff(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.changes = staged("a.go", "b.go")
	world.diff = []string{"+x"}

	// Act
	typing(t, world.live(t, 120, 40), "3", "j")

	// Assert
	if calls := world.asked("diff b.go"); len(calls) != 1 {
		t.Errorf("diff reads = %q, want one for the newly selected file", world.asked("diff"))
	}
}

func TestTheDiffScrolls(t *testing.T) {
	t.Parallel()

	// Arrange
	lines := make([]string, 0, 61)
	for range 60 {
		lines = append(lines, "+a changed line")
	}

	lines = append(lines, "+THE END")

	world := newWorld()
	world.diff = lines

	// Act
	scrolled := typing(t, world.live(t, 120, 30), "3", "pgdown", "pgdown", "pgdown", "pgdown")

	// Assert
	requireScreen(t, scrolled.View().Content, "THE END")
}

func TestAFailedDiffIsShownInPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.diffErr = errDiffRead

	// Act
	view := typing(t, world.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view, "diff: "+redactPath, errDiffRead.Error())
}

func TestTheDiffReadsAsItLoads(t *testing.T) {
	t.Parallel()

	// Arrange
	// The loads the interface starts with are answered, but not the diff read the
	// changes lead to, so the pane is caught mid-load.
	world := newWorld()
	model := started(t, sized(t, tui.New(world.cfg, nil, world.deps()), 120, 40))

	// Act
	view := press(t, model, "3").View().Content

	// Assert
	requireScreen(t, view, "reading the diff")
}

func TestWithoutADiffSeamNoDiffShows(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.noDiff = true
	world.diff = []string{"+should not show"}

	// Act
	view := typing(t, world.live(t, 120, 40), "3").View().Content

	// Assert
	refuseScreen(t, view, "diff:", "should not show")
}

func TestBackOnAFileWhoseDiffFailedReadsItAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// The first file's diff fails to read. The selection moves on, and back
	// before the next file's diff answers.
	world := newWorld()
	world.changes = staged("a.go", "b.go")
	world.diffErr = errDiffRead
	failed := typing(t, world.live(t, 120, 40), "3")
	world.diffErr = nil
	world.diff = []string{"+read again"}
	moved, late := withheldKeys(t, failed, world.counting("diff "), "j")
	back := typing(t, moved, "k")

	// Act
	view := handedOver(t, back, late).View().Content

	// Assert
	requireScreen(t, view, "diff: a.go", "+read again")
	refuseScreen(t, view, errDiffRead.Error())
}

func TestStayingOnAShownDiffReadsItNoMore(t *testing.T) {
	t.Parallel()

	// Arrange
	// The default world has one file changed, so j stays on it.
	world := newWorld()
	shown := typing(t, world.live(t, 120, 40), "3")

	// Act
	typing(t, shown, "j")

	// Assert
	if calls := world.asked("diff "); len(calls) != 1 {
		t.Errorf("diff reads = %q, want the one that showed it", calls)
	}
}

func TestStayingOnADiffStillReadingReadsItNoMore(t *testing.T) {
	t.Parallel()

	// Arrange
	// A refresh reads the file's diff again, slowly.
	world := newWorld()
	reading, _ := withheldKeys(t, typing(t, world.live(t, 120, 40), "3"), world.counting("diff "), "r")

	// Act
	typing(t, reading, "j")

	// Assert
	if calls := world.asked("diff "); len(calls) != 2 {
		t.Errorf("diff reads = %q, want the first and the refresh's, still out", calls)
	}
}

func TestStayingOnAFailedDiffReadsItAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.diffErr = errDiffRead
	failed := typing(t, world.live(t, 120, 40), "3")
	world.diffErr = nil
	world.diff = []string{"+read again"}

	// Act
	view := typing(t, failed, "j").View().Content

	// Assert
	requireScreen(t, view, "diff: "+redactPath, "+read again")
	refuseScreen(t, view, errDiffRead.Error())
}

func TestAFailedDiffReadAgainSaysItIsReading(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.diffErr = errDiffRead
	failed := typing(t, world.live(t, 120, 40), "3")

	// Act
	reading, _ := withheldKeys(t, failed, world.counting("diff "), "j")

	// Assert
	requireScreen(t, reading.View().Content, "reading the diff")
	refuseScreen(t, reading.View().Content, errDiffRead.Error())
}
