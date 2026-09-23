// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"
)

// errRebaseConflict is git stopping a rebase on a conflict: a non-zero exit,
// shaped like the exit statuses the run overlay turns into a headline.
var errRebaseConflict = errors.New("exit status 1")

func TestCatchingUpIsOfferedAgainstTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	behind := newWorld()

	// Act
	view := typing(t, behind.live(t, 120, 40), "2").View().Content

	// Assert
	requireScreen(t, footerLine(view), "u rebase onto base")
}

func TestCatchingUpIsNotOfferedWithoutABaseToCatchUpWith(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*world){
		"on the base branch": func(w *world) { w.branch.Name = baseName },
		"no base found":      func(w *world) { w.branch.Base = "" },
	}

	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			settled := newWorld()
			arrange(settled)
			model := settled.live(t, 120, 40)

			// Act: open the Branch pane
			pane := typing(t, model, "2")

			// Assert: catching up is not offered
			refuseScreen(t, footerLine(pane.View().Content), "u rebase onto base")

			// Act: press it anyway, and confirm
			typing(t, pane, "u", keyEnter)

			// Assert: a key not offered does nothing
			if calls := settled.asked("rebase"); len(calls) != 0 {
				t.Errorf("u rebased with no base to catch up with: %q", calls)
			}
		})
	}
}

func TestCatchingUpReplaysOntoTheBase(t *testing.T) {
	t.Parallel()

	// Arrange
	behind := newWorld()
	behind.rebaseLines = []string{"Successfully rebased and updated refs/heads/" + featureName + "."}

	// Act
	done := typing(t, behind.live(t, 120, 40), "2", "u", keyEnter)

	// Assert
	requireScreen(t, done.View().Content, "● rebased onto "+baseRef)

	if calls := behind.asked("rebase"); len(calls) != 1 || calls[0] != "rebase "+baseRef {
		t.Errorf("rebase calls = %q, want a rebase onto the base", calls)
	}
}

func TestARebaseConflictIsLeftForTheShell(t *testing.T) {
	t.Parallel()

	// Arrange
	conflicting := newWorld()
	conflicting.rebaseLines = []string{"CONFLICT (content): Merge conflict in internal/config/redact.go"}
	conflicting.rebaseErr = errRebaseConflict

	// Act
	stopped := typing(t, conflicting.live(t, 120, 40), "2", "u", keyEnter)

	// Assert
	requireScreen(t, stopped.View().Content, "┏━ git rebase", "✗ the rebase stopped",
		"CONFLICT (content): Merge conflict in internal/config/redact.go")
}

func TestADryRunRebaseIsOnlyDescribed(t *testing.T) {
	t.Parallel()

	// Arrange
	dry := newWorld()
	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	// Act
	view := typing(t, model, "2", "u", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "dry run: would rebase onto "+baseRef)
	refuseScreen(t, view, "Rebase branch")

	if calls := dry.asked("rebase"); len(calls) != 0 {
		t.Errorf("a dry run rebased: %q", calls)
	}
}

func TestRebaseAsksBeforeTheRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	behind := newWorld()
	behind.rebaseLines = []string{"Successfully rebased and updated refs/heads/" + featureName + "."}
	onBranch := typing(t, behind.live(t, 120, 40), "2")

	// Act: press u
	look := typing(t, onBranch, "u")

	// Assert: the look names the branch and its base, and nothing is run yet
	requireScreen(t, look.View().Content,
		"Rebase branch", "rebase "+featureName+" onto "+baseRef, "enter rebase", "esc close")

	if calls := behind.asked("rebase"); len(calls) != 0 {
		t.Fatalf("rebased before the look was confirmed: %q", calls)
	}

	// Act: back out
	backedOut := typing(t, look, keyEsc)

	// Assert: the look closes, and esc runs nothing
	refuseScreen(t, backedOut.View().Content, "Rebase branch")

	if calls := behind.asked("rebase"); len(calls) != 0 {
		t.Fatalf("esc rebased the branch: %q", calls)
	}

	// Act: open it again and go ahead
	done := typing(t, backedOut, "u", keyEnter)

	// Assert: enter rebases onto the base, once
	if calls := behind.asked("rebase"); len(calls) != 1 || calls[0] != "rebase "+baseRef {
		t.Errorf("rebase calls = %q, want one rebase onto the base", calls)
	}

	requireScreen(t, done.View().Content, "● rebased onto "+baseRef)
}
