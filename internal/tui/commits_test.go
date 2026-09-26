// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"slices"
	"strconv"
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
		{Path: redactPath, Staged: 'M', Unstaged: ' '},
		{Path: "internal/log/debug.go", Staged: 'M', Unstaged: 'M'},
		{Path: untrackedNotes, Staged: '?', Unstaged: '?'},
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
	view := typing(t, changing.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view,
		"▸ ● modified   internal/config/redact.go", "◐ modified   internal/log/debug.go", "○ untracked  notes.txt",
		"● renamed    old.go → new.go", "✗ conflicted conflict.go", "On this branch", "1a2b3c4 "+pullTitle,
		"3 of 6 staged", "1 commit on this branch")

	// A file name is anyone's to choose, and an escape sequence in one is
	// neutralized before it reaches the terminal. lipgloss colors the screen with
	// its own CSI (\x1b[…) escapes, so the injected OSC (\x1b]…) and its BEL
	// terminator are what prove the file name was neutralized.
	if strings.Contains(view, "\x1b]") || strings.ContainsRune(view, 0x07) {
		t.Errorf("a file name drove the terminal:\n%q", view)
	}
}

func TestACleanTreeSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	clean := newWorld()
	clean.changes = nil

	// Act
	view := typing(t, clean.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view, "nothing changed")
	refuseScreen(t, footerLine(view), "space stage", "c commit")
}

func TestTheCommitsDetailWaitsForTheStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	// A model whose status has not loaded yet: Init has not been drained.
	model := sized(t, tui.New(completeConfig(), nil, newWorld().deps()), 120, 40)

	// Act
	view := press(t, model, "3").View().Content

	// Assert
	// It says it is loading, not that nothing changed.
	requireScreen(t, view, "loading")
	refuseScreen(t, view, "nothing changed")
}

func TestAStatusThatCannotBeReadSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld().deps()
	failing.Git.Changes = func() ([]gitrepo.Change, error) { return nil, errLocked }
	model := sized(t, tui.New(completeConfig(), nil, failing), 120, 40)

	// Act
	view := typing(t, drain(t, model, model.Init()), "3").View().Content

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

func TestTheCommitsFooterOffersStageAllOnlyWithAFileLeftToStage(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		changes []gitrepo.Change
		offered bool
	}{
		"every file staged": {
			changes: []gitrepo.Change{{Path: redactPath, Staged: 'M', Unstaged: ' '}},
		},
		"a file left to stage": {changes: workTree(), offered: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			staging := newWorld()
			staging.changes = tt.changes

			// Act
			footer := footerLine(typing(t, staging.live(t, 200, 40), "3").View().Content)

			// Assert
			if offered := strings.Contains(footer, "a stage all"); offered != tt.offered {
				t.Errorf("the footer offers a stage all: %t, want %t:\n%s", offered, tt.offered, footer)
			}
		})
	}
}

func TestAStagingFailureIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	locked := newWorld()
	locked.stageErr = errLocked

	// Act
	view := typing(t, locked.live(t, 120, 40), "3", keySpace).View().Content

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
			view := typing(t, model, tt.keys...).View().Content

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
	requireScreen(t, moved.View().Content, "▸ ◐ modified   internal/log/debug.go")

	// Act: click the third file
	// Row 4 of the detail is the third file: the detail starts at row 1, and
	// its border takes a row.
	clicked := click(t, moved, 60, 4)

	// Assert: it is selected
	requireScreen(t, clicked.View().Content, "▸ ○ untracked  notes.txt")

	// Act: click below the files
	below := click(t, clicked, 60, 30)

	// Assert: nothing is picked
	if below.View().Content != clicked.View().Content {
		t.Errorf("a click below the files changed the screen:\n%s", below.View().Content)
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
	view := typing(t, odd.live(t, 120, 40), "3").View().Content

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
	view := typing(t, sequenced.live(t, 120, 40), "3", keySpace).View().Content

	// Assert
	requireScreen(t, view, "✗ cannot add this file")
	refuseScreen(t, view, "owned")
}

func TestTheCommitsListKeepsTheSelectedFileOnScreen(t *testing.T) {
	t.Parallel()

	// Arrange
	crowded := newWorld()
	crowded.changes = make([]gitrepo.Change, 60)

	for index := range crowded.changes {
		crowded.changes[index] = gitrepo.Change{Path: "file" + strconv.Itoa(index) + ".go", Staged: 'M', Unstaged: ' '}
	}

	down := append([]string{"3"}, slices.Repeat([]string{"j"}, 40)...)

	// Act
	view := typing(t, crowded.live(t, 120, 20), down...).View().Content

	// Assert
	requireScreen(t, view, "▸ ● modified   file40.go")
}

func TestClickingAfterTheTreeShrinksStagesTheVisibleFile(t *testing.T) {
	t.Parallel()

	// Arrange
	// Scroll a long file list, then let a reload return a much shorter one. A click
	// must count from the rows drawn, not from an offset the longer list left.
	shrinking := newWorld()
	shrinking.changes = changedFiles("old", 20)

	down := append([]string{"3"}, slices.Repeat([]string{"j"}, 19)...)
	scrolled := typing(t, shrinking.live(t, 120, 20), down...)

	shrinking.changes = []gitrepo.Change{
		{Path: "a.go", Staged: ' ', Unstaged: 'M'},
		{Path: "b.go", Staged: ' ', Unstaged: 'M'},
		{Path: "c.go", Staged: ' ', Unstaged: 'M'},
	}
	reloaded := typing(t, scrolled, "r")

	// Act
	// Row 3 of the detail is the second file (b.go); stage it after the click.
	typing(t, click(t, reloaded, 60, 3), keySpace)

	// Assert
	// The click reached b.go, the file drawn there. Counted from the longer list's
	// offset, it maps past the end, and space stages a.go.
	if got := shrinking.asked("stage b.go"); len(got) != 1 {
		t.Errorf("stage calls = %v, want the clicked visible file b.go", shrinking.asked("stage"))
	}
}

func TestTheCommitsPaneNamesEachKindOfChangeInWords(t *testing.T) {
	t.Parallel()

	// Arrange
	changing := newWorld()
	changing.changes = workTree()

	// Act
	view := typing(t, changing.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view,
		"modified   internal/config/redact.go",
		"untracked  notes.txt",
		"renamed    old.go → new.go",
		"conflicted conflict.go",
		"3 of 6 staged")
	refuseScreen(t, view, "M  internal/config", "?? notes.txt", "3 staged · 6 changed")
}

func TestTheHeavyBorderFollowsTheCursorIntoTheCommitsDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	changing := newWorld()
	changing.changes = workTree()

	// Act
	view := typing(t, changing.live(t, 120, 40), "3").View().Content

	// Assert
	requireScreen(t, view, "┏━ Commits ")

	if heavy := strings.Count(view, "┏"); heavy != 1 {
		t.Errorf("found %d heavy top-left corners, want exactly one:\n%s", heavy, view)
	}
}

func TestANoticeWithANewlineDoesNotOverflowTheScreen(t *testing.T) {
	t.Parallel()

	// Arrange
	// A staging failure can carry a program's stderr, which is several lines.
	noisy := newWorld()
	//nolint:err113 // a one-off multi-line error is the fixture
	noisy.stageErr = errors.New("fatal: could not stage\nhint: the index is locked\nhint: remove .git/index.lock")

	// Act
	view := typing(t, noisy.live(t, 120, 40), "3", keySpace).View().Content

	// Assert
	if rows := strings.Count(view, "\n") + 1; rows > 40 {
		t.Errorf("View is %d rows for a %d-row terminal; a multi-line notice overflowed:\n%q", rows, 40, view)
	}
}
