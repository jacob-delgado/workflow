// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// errLocked stands in for git refusing to touch a locked index.
var errLocked = errors.New("fatal: Unable to create '.git/index.lock': File exists")

// workTree is a work tree with every kind of change.
func workTree() []gitrepo.Change {
	return []gitrepo.Change{
		{Path: "internal/config/redact.go", Staged: 'M', Unstaged: ' '},
		{Path: "internal/log/debug.go", Staged: 'M', Unstaged: 'M'},
		{Path: "notes.txt", Staged: '?', Unstaged: '?'},
		{Path: "new.go", OriginalPath: "old.go", Staged: 'R', Unstaged: ' '},
		{Path: "conflict.go", Staged: 'U', Unstaged: 'U'},
		{Path: "evil\x1b]0;owned\x07.go", Staged: '?', Unstaged: '?'},
	}
}

func TestTheCommitsPaneShowsWhereEachFileStands(t *testing.T) {
	t.Parallel()

	// Arrange
	changing := newWorld()
	changing.changes = workTree()

	// Act
	view := typing(t, changing.live(t, 120, 40), "3").View()

	// Assert
	requireScreen(t, view,
		"▸ ● M  internal/config/redact.go", "◐ MM internal/log/debug.go", "○ ?? notes.txt",
		"● R  old.go → new.go", "✗ UU conflict.go", "On this branch", "1a2b3c4 "+pullTitle,
		"3 staged · 6 changed", "1 commit on this branch")

	// A file name is anyone's to choose, and an escape sequence in one is
	// neutralized before it reaches the terminal.
	if strings.ContainsRune(view, 0x1b) {
		t.Errorf("a file name drove the terminal:\n%q", view)
	}
}

func TestACleanTreeSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	clean := newWorld()
	clean.changes = nil

	// Act
	view := typing(t, clean.live(t, 120, 40), "3").View()

	// Assert
	requireScreen(t, view, "nothing changed")
	refuseScreen(t, footerLine(view), "space stage", "c commit")
}

func TestAStatusThatCannotBeReadSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld().deps()
	failing.Git.Changes = func() ([]gitrepo.Change, error) { return nil, errLocked }
	model := sized(t, tui.New(completeConfig(), nil, failing), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "3").View()

	// Assert
	requireScreen(t, view, "status failed", "✗ fatal: Unable to create")
}

func TestSpaceStagesOrUnstagesTheSelectedFile(t *testing.T) {
	t.Parallel()

	// Arrange
	staging := newWorld()
	staging.changes = workTree()
	model := staging.live(t, 120, 40)

	// Act
	// The first is wholly staged, so space unstages it; the second is partly
	// staged, so space stages the rest; the third is untracked.
	typing(t, model, "3", keySpace, "j", keySpace, "j", keySpace)

	// Assert
	want := []string{"unstage internal/config/redact.go", "stage internal/log/debug.go", "stage notes.txt"}

	got := append(staging.asked("unstage"), staging.asked("stage")...)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("staging calls = %q, want %q", got, want)
	}

	if reads := staging.asked("changes"); len(reads) != 4 {
		t.Errorf("read the status %d times, want once at start and again after each change", len(reads))
	}
}

func TestStageAllStagesEverythingNotYetStaged(t *testing.T) {
	t.Parallel()

	// Arrange
	staging := newWorld()
	staging.changes = workTree()

	// Act
	typing(t, staging.live(t, 120, 40), "3", "a")

	// Assert
	want := "stage internal/log/debug.go|stage notes.txt|stage conflict.go|stage evil\x1b]0;owned\x07.go"
	if got := strings.Join(staging.asked("stage"), "|"); got != want {
		t.Errorf("stage calls = %q, want %q", got, want)
	}
}

func TestStageAllWithEverythingStagedStagesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	nothing := newWorld()

	// Act
	typing(t, nothing.live(t, 120, 40), "3", "a")

	// Assert
	if calls := nothing.asked("stage"); len(calls) != 0 {
		t.Errorf("stage all staged what was staged already: %q", calls)
	}
}

func TestAStagingFailureIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	locked := newWorld()
	locked.stageErr = errLocked

	// Act
	view := typing(t, locked.live(t, 120, 40), "3", keySpace).View()

	// Assert
	requireScreen(t, view, "✗ fatal: Unable to create")
}

func TestADryRunStagesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"staging a file":   {keys: []string{"3", "j", keySpace}, want: "dry run: would stage internal/log/debug.go"},
		"unstaging a file": {keys: []string{"3", keySpace}, want: "dry run: would unstage internal/config/redact.go"},
		"staging all":      {keys: []string{"3", "a"}, want: "dry run: would stage 4 files"},
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
			view := typing(t, model, tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)

			if calls := append(dry.asked("stage"), dry.asked("unstage")...); len(calls) != 0 {
				t.Errorf("a dry run staged: %q", calls)
			}
		})
	}
}

func TestTheSelectionMovesAndCanBeClicked(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.changes = workTree()
	model := choosing.live(t, 120, 40)

	// Act: move down twice and back up
	moved := typing(t, model, "3", "j", "j", "k")

	// Assert: the second file is selected
	requireScreen(t, moved.View(), "▸ ◐ MM internal/log/debug.go")

	// Act: click the third file
	// Row 4 of the detail is the third file: the detail starts at row 1, and
	// its border takes a row.
	clicked := click(t, moved, 60, 4)

	// Assert: it is selected
	requireScreen(t, clicked.View(), "▸ ○ ?? notes.txt")

	// Act: click below the files
	below := click(t, clicked, 60, 30)

	// Assert: nothing is picked
	if below.View() != clicked.View() {
		t.Errorf("a click below the files changed the screen:\n%s", below.View())
	}
}

func TestAFileNameIsDrawnOnOneLine(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file name may hold a newline, and a name drawn over two rows is a row
	// that names no file.
	odd := newWorld()
	odd.changes = []gitrepo.Change{{Path: "one\ntwo.go", Staged: ' ', Unstaged: 'M'}}

	// Act
	view := typing(t, odd.live(t, 120, 40), "3").View()

	// Assert
	requireScreen(t, view, "one\ufffdtwo.go")
}

func TestAFailureIsShownInTextThatIsSafe(t *testing.T) {
	t.Parallel()

	// Arrange
	// An error carries what a program or a file name put in it.
	sequenced := newWorld()
	//nolint:err113 // a one-off error text is the fixture
	sequenced.stageErr = errors.New("cannot add \x1b]0;owned\x07this file")

	// Act
	view := typing(t, sequenced.live(t, 120, 40), "3", keySpace).View()

	// Assert
	requireScreen(t, view, "✗ cannot add this file")
	refuseScreen(t, view, "owned")
}
