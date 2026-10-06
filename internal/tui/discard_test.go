// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

func TestUnstageAllTakesEveryStagedChangeOutOfTheIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	staging := newWorld()
	staging.changes = workTree()

	// Act
	typing(t, staging.live(t, 120, 40), "3", "U")

	// Assert
	// The conflict and the untracked files hold nothing in the index.
	want := "unstage internal/config/redact.go|unstage internal/log/debug.go|unstage new.go"
	if got := strings.Join(staging.asked("unstage"), "|"); got != want {
		t.Errorf("unstage calls = %q, want %q", got, want)
	}

	if reads := staging.asked("changes"); len(reads) != 2 {
		t.Errorf("read the status %d times, want once at start and again after unstaging", len(reads))
	}
}

func TestTheCommitsFooterOffersUnstageAllOnlyWithSomethingStaged(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		changes []gitrepo.Change
		offered bool
	}{
		"nothing staged": {changes: []gitrepo.Change{{Path: untrackedNotes, Staged: '?', Unstaged: '?'}}},
		"a file staged":  {changes: workTree(), offered: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			staging := newWorld()
			staging.changes = tt.changes

			// Act
			model := typing(t, staging.live(t, 240, 40), "3", "U")

			// Assert
			footer := footerLine(model.View().Content)
			if offered := strings.Contains(footer, "U unstage all"); offered != tt.offered {
				t.Errorf("the footer offers unstage all: %t, want %t:\n%s", offered, tt.offered, footer)
			}

			if unstaged := len(staging.asked("unstage")) > 0; unstaged != tt.offered {
				t.Errorf("U unstaged: %t, want %t", unstaged, tt.offered)
			}
		})
	}
}

func TestDiscardAsksALastLookBeforeDroppingTheFile(t *testing.T) {
	t.Parallel()

	// Arrange
	discarding := newWorld()
	discarding.changes = workTree()
	discarding.diff = []string{"-\tkeep := true", "+\tkeep := false"}
	model := typing(t, discarding.live(t, 120, 40), "3", "j")

	// Act: open the last look
	looking := typing(t, model, "x")

	// Assert: it shows the file and its change, says it cannot be undone, and
	// nothing is discarded yet
	requireScreen(t, looking.View().Content, "Discard changes", "internal/log/debug.go", "keep := false",
		"cannot be undone", "enter discard")

	if calls := discarding.asked("discard"); len(calls) != 0 {
		t.Fatalf("discarded before the look was confirmed: %q", calls)
	}

	// Act: confirm it
	view := typing(t, looking, keyEnter).View().Content

	// Assert: that file alone is discarded, said, and the status read again
	if got := discarding.asked("discard"); len(got) != 1 || got[0] != "discard internal/log/debug.go" {
		t.Errorf("discard calls = %q, want the selected file's alone", got)
	}

	requireScreen(t, view, "discarded internal/log/debug.go")
	refuseScreen(t, view, "cannot be undone")

	if reads := discarding.asked("changes"); len(reads) != 2 {
		t.Errorf("read the status %d times, want once at start and again after discarding", len(reads))
	}
}

func TestEscLeavesTheFileAsItIs(t *testing.T) {
	t.Parallel()

	// Arrange
	keeping := newWorld()
	keeping.changes = workTree()

	// Act
	view := typing(t, keeping.live(t, 120, 40), "3", "x", keyEsc).View().Content

	// Assert
	refuseScreen(t, view, "cannot be undone")

	if calls := keeping.asked("discard"); len(calls) != 0 {
		t.Errorf("esc discarded: %q", calls)
	}
}

func TestARefusedDiscardStaysInTheLookAndSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	locked := newWorld()
	locked.changes = workTree()
	locked.discardErr = errLocked

	// Act
	view := typing(t, locked.live(t, 120, 40), "3", "x", keyEnter).View().Content

	// Assert
	requireScreen(t, view, "Discard changes", "✗ fatal: Unable to create")
}

func TestADryRunDiscardsAndUnstagesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"unstaging all":     {keys: []string{"3", "U"}, want: "dry run: would unstage 3 files"},
		"discarding a file": {keys: []string{"3", "x", keyEnter}, want: "dry run: would discard internal/config/redact.go"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dry := newWorld()
			dry.changes = workTree()
			model := sized(t, dryInterface(dry), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, tt.keys...).View().Content

			// Assert
			requireScreen(t, view, tt.want)

			if calls := append(dry.asked("discard"), dry.asked("unstage")...); len(calls) != 0 {
				t.Errorf("a dry run wrote: %q", calls)
			}
		})
	}
}

func TestNothingToUnstageOrDiscardDoesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(w *world) tui.Model{
		"a clean tree": func(w *world) tui.Model {
			w.changes = nil

			return w.live(t, 120, 40)
		},
		"no way to write": func(w *world) tui.Model {
			w.changes = workTree()
			model := sized(t, tui.New(w.cfg, nil, readOnly(w)), 120, 40)

			return drain(t, model, model.Init())
		},
	}

	for name, start := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			idle := newWorld()
			model := start(idle)

			// Act
			view := typing(t, model, "3", "U", "x").View().Content

			// Assert
			refuseScreen(t, view, "Discard changes")

			if calls := append(idle.asked("discard"), idle.asked("unstage")...); len(calls) != 0 {
				t.Errorf("wrote with nothing to write: %q", calls)
			}
		})
	}
}

func TestTheDiscardLookNamesTheFileWithoutADiffToShow(t *testing.T) {
	t.Parallel()

	// Arrange
	undiffed := newWorld()
	undiffed.changes = workTree()
	undiffed.noDiff = true

	// Act
	view := typing(t, undiffed.live(t, 120, 40), "3", "x").View().Content

	// Assert
	requireScreen(t, view, "Discard the modified file internal/config/redact.go", "cannot be undone")
}
