// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestStoppingARunningCommandSaysStoppingUntilItsExitArrives(t *testing.T) {
	t.Parallel()

	// Arrange
	// A pre-commit hook that hangs: it streams nothing and never finishes.
	w := newWorld()
	w.runBlocks = true
	picking := typing(t, w.live(t, 100, 40), "3", "h")

	// Act: start the hook, which does not finish on its own, so its output is
	// never read.
	starting, start := pressed(t, picking, keyEnter)
	running, readOutput := finish(t, starting, start)

	// Assert: the run is shown as still going.
	requireScreen(t, running.View().Content, "running")

	// Act: stop it.
	stopping := typing(t, running, "s")

	// Assert: the run's own Stop was called, and the screen says it is
	// stopping, no longer offering the stop it was given.
	if w.runStop == nil || !*w.runStop {
		t.Error("the stop key did not stop the run")
	}

	requireScreen(t, stopping.View().Content, "stopping")
	refuseScreen(t, footerLine(stopping.View().Content), "s stop")

	// Act: the killed program's exit arrives.
	stopped := drain(t, stopping, readOutput).View().Content

	// Assert: the screen says it stopped.
	requireScreen(t, stopped, "stopped")
	refuseScreen(t, stopped, "stopping")
}

func TestAStoppedCommitThatExitsCleanlyIsTakenForCommitted(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit's hooks hang; it is stopped, and the program exits cleanly:
	// it finished before the stop reached it, since a stop kills the process
	// group, which no program passes through.
	w := newWorld()
	w.runBlocks, w.stopExitsClean = true, true
	composing := typing(t, w.live(t, 100, 40), append([]string{"3", "c"}, letters("x")...)...)
	starting, start := pressed(t, composing, keyEnter)
	running, readOutput := finish(t, starting, start)
	stopped := typing(t, running, "s")

	// Act
	view := drain(t, stopped, readOutput).View().Content

	// Assert
	requireScreen(t, view, "committed fix: x")
	refuseScreen(t, view, "stopped")
}

func TestAJobStillRunningWhenItsRunIsStoppedIsNoLongerShownAsGoing(t *testing.T) {
	t.Parallel()

	// Arrange
	// The hook starts a lint job and hangs inside it, so lefthook never reports it.
	w := newWorld()
	w.runBlocks, w.commitLines = true, []string{lintJobStarts}
	starting, start := pressed(t, typing(t, w.live(t, 100, 40), "3", "h"), keyEnter)
	running, readOutput := finish(t, starting, start)
	streamed, readRest := finish(t, running, readOutput)

	// Act
	stopped := drain(t, typing(t, streamed, "s"), readRest).View().Content

	// Assert
	requireScreen(t, stopped, "○ stopped", "○ lint")
	refuseScreen(t, stopped, "◐ lint")
}

func TestAStoppedHookThatExitsCleanlyIsShownDone(t *testing.T) {
	t.Parallel()

	// Arrange
	// The hook hangs; it is stopped, and the program exits cleanly, having
	// finished before the stop reached it.
	w := newWorld()
	w.runBlocks, w.stopExitsClean = true, true
	starting, start := pressed(t, typing(t, w.live(t, 100, 40), "3", "h"), keyEnter)
	running, readOutput := finish(t, starting, start)
	stopping := typing(t, running, "s")

	// Act
	view := drain(t, stopping, readOutput).View().Content

	// Assert
	requireScreen(t, view, "● done")
	refuseScreen(t, view, "stopped")
}
