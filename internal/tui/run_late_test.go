// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// runAnswers is every answer a failed hook run gave, in the order it gave
// them, kept so a test can hand one over again once the run is retried.
type runAnswers struct {
	start, line, exit, places tea.Msg
}

// failedRunAnswers runs a failing lint hook to its places, keeping its start,
// its first line, its exit and its places, and retries it: the run that
// answered is gone, and the retry, not yet started, is the one shown.
func failedRunAnswers(t *testing.T) (tui.Model, runAnswers) {
	t.Helper()

	failing := failingLint()
	running, start := pressed(t, typing(t, failing.live(t, 120, 40), "3", "h"), keyEnter)
	kept := runAnswers{start: start()}
	running, next := takes(t, running, kept.start)
	kept.line = next()
	running, next = takes(t, running, kept.line)

	for range len(failing.commitLines) - 1 {
		running, next = takes(t, running, next())
	}

	kept.exit = next()
	failed, finding := takes(t, running, kept.exit)
	kept.places = finding()
	offered, _ := takes(t, failed, kept.places)
	retried, _ := pressed(t, offered, "r")

	return retried, kept
}

// takes hands the model one answer, as Bubble Tea does once the work
// behind it returns.
func takes(t *testing.T, model tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	t.Helper()

	next, cmd := model.Update(msg)

	return concrete(t, next), cmd
}

func TestAnAnswerForARunSinceRetriedChangesNothing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		answer func(runAnswers) tea.Msg
	}{
		{name: "its start", answer: func(kept runAnswers) tea.Msg { return kept.start }},
		{name: "a line", answer: func(kept runAnswers) tea.Msg { return kept.line }},
		{name: "its exit", answer: func(kept runAnswers) tea.Msg { return kept.exit }},
		{name: "its places", answer: func(kept runAnswers) tea.Msg { return kept.places }},
	}

	for _, answering := range cases {
		t.Run(answering.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			retried, kept := failedRunAnswers(t)
			before := retried.View().Content

			// Act
			late, _ := takes(t, retried, answering.answer(kept))

			// Assert
			if after := late.View().Content; after != before {
				t.Errorf("an answer for the run retried changed the retry's screen:\n%s\nwant it as it was:\n%s",
					plain(after), plain(before))
			}

			refuseScreen(t, footerLine(late.View().Content), "s stop")
		})
	}
}

func TestAKeyOtherThanStopLeavesARunGoing(t *testing.T) {
	t.Parallel()

	// Arrange
	w := newWorld()
	w.runBlocks = true
	starting, start := pressed(t, typing(t, w.live(t, 100, 40), "3", "h"), keyEnter)
	running, _ := finish(t, starting, start)

	// Act
	pressedOther := typing(t, running, "r")

	// Assert
	if w.runStop == nil || *w.runStop {
		t.Error("a key other than stop stopped the run")
	}

	requireScreen(t, pressedOther.View().Content, "running")
	requireScreen(t, footerLine(pressedOther.View().Content), "s stop")
}
