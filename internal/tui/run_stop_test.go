// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import "testing"

func TestStoppingARunningCommandEndsItAndSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	// A pre-commit hook that hangs: it streams nothing and never finishes.
	w := newWorld()
	w.runBlocks = true
	commits := typing(t, w.live(t, 100, 40), "3")

	// Act: start the hook, which does not finish on its own, so its output is
	// never read.
	starting, start := pressed(t, commits, "h")
	running, _ := finish(t, starting, start)

	// Assert: the run is shown as still going.
	requireScreen(t, running.View().Content, "running")

	// Act: stop it.
	stopped := typing(t, running, "s")

	// Assert: the run's own Stop was called, and the screen says it stopped.
	if w.runStop == nil || !*w.runStop {
		t.Error("the stop key did not stop the run")
	}

	requireScreen(t, stopped.View().Content, "stopped")
}

func TestAStoppedCommitThatExitsCleanlyIsNotTakenForCommitted(t *testing.T) {
	t.Parallel()

	// Arrange
	// The commit's hooks hang; it is stopped, and the program, trapping the
	// signal, exits as if it had finished.
	w := newWorld()
	w.runBlocks, w.stopExitsClean = true, true
	composing := typing(t, w.live(t, 100, 40), append([]string{"3", "c"}, letters("x")...)...)
	starting, start := pressed(t, composing, keyEnter)
	running, readOutput := finish(t, starting, start)
	stopped := typing(t, running, "s")

	// Act
	view := drain(t, stopped, readOutput).View().Content

	// Assert
	requireScreen(t, view, "stopped")
	refuseScreen(t, view, "committed fix: x")
}
