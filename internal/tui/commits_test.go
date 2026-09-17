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

// errHookFailed is how git reports a hook that failed.
var errHookFailed = errors.New("exit status 1")

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

	changing := newWorld()
	changing.changes = workTree()

	view := typing(t, changing.live(t, 120, 40), "3").View()
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

	clean := newWorld()
	clean.changes = nil

	view := typing(t, clean.live(t, 120, 40), "3").View()
	requireScreen(t, view, "nothing changed")
	refuseScreen(t, footerLine(view), "space stage", "c commit")

	failing := newWorld().deps()
	failing.Git.Changes = func() ([]gitrepo.Change, error) { return nil, errLocked }

	model := sized(t, tui.New(completeConfig(), nil, failing), 120, 40)
	requireScreen(t, typing(t, drain(t, model, model.Init()), "3").View(), "status failed", "✗ fatal: Unable to create")
}

func TestSpaceStagesOrUnstagesTheSelectedFile(t *testing.T) {
	t.Parallel()

	staging := newWorld()
	staging.changes = workTree()

	// The first is wholly staged, so space unstages it; the second is partly
	// staged, so space stages the rest; the third is untracked.
	typing(t, staging.live(t, 120, 40), "3", "space", "j", "space", "j", "space")

	want := []string{"unstage internal/config/redact.go", "stage internal/log/debug.go", "stage notes.txt"}

	got := append(staging.asked("unstage"), staging.asked("stage")...)

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("staging calls = %q, want %q", got, want)
	}

	if reads := staging.asked("changes"); len(reads) < 4 {
		t.Errorf("read the status %d times, want it read again after each change", len(reads))
	}
}

func TestStageAllStagesEverythingNotYetStaged(t *testing.T) {
	t.Parallel()

	staging := newWorld()
	staging.changes = workTree()

	typing(t, staging.live(t, 120, 40), "3", "a")

	want := "stage internal/log/debug.go|stage notes.txt|stage conflict.go|stage evil\x1b]0;owned\x07.go"
	if got := strings.Join(staging.asked("stage"), "|"); got != want {
		t.Errorf("stage calls = %q, want %q", got, want)
	}

	nothing := newWorld()
	typing(t, nothing.live(t, 120, 40), "3", "a")

	if calls := nothing.asked("stage"); len(calls) != 0 {
		t.Errorf("stage all staged what was staged already: %q", calls)
	}
}

func TestAStagingFailureIsReported(t *testing.T) {
	t.Parallel()

	locked := newWorld()
	locked.stageErr = errLocked

	requireScreen(t, typing(t, locked.live(t, 120, 40), "3", "space").View(), "✗ fatal: Unable to create")
}

func TestADryRunStagesNothing(t *testing.T) {
	t.Parallel()

	dry := newWorld()
	dry.changes = workTree()

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	requireScreen(t, typing(t, model, "3", "j", "space").View(), "dry run: would stage internal/log/debug.go")
	requireScreen(t, typing(t, model, "3", "space").View(), "dry run: would unstage internal/config/redact.go")
	requireScreen(t, typing(t, model, "3", "a").View(), "dry run: would stage 4 files")

	if calls := append(dry.asked("stage"), dry.asked("unstage")...); len(calls) != 0 {
		t.Errorf("a dry run staged: %q", calls)
	}
}

func TestTheSelectionMovesAndCanBeClicked(t *testing.T) {
	t.Parallel()

	choosing := newWorld()
	choosing.changes = workTree()

	moved := typing(t, choosing.live(t, 120, 40), "3", "j", "j", "k")
	requireScreen(t, moved.View(), "▸ ◐ MM internal/log/debug.go")

	// Row 4 of the detail is the third file: the detail starts at row 1, and
	// its border takes a row.
	clicked := click(t, moved, 60, 4)
	requireScreen(t, clicked.View(), "▸ ○ ?? notes.txt")

	// A click below the files picks nothing.
	requireScreen(t, click(t, clicked, 60, 30).View(), "▸ ○ ?? notes.txt")
}

func TestCommitNeedsSomethingStaged(t *testing.T) {
	t.Parallel()

	unstaged := newWorld()
	unstaged.changes = []gitrepo.Change{{Path: "notes.txt", Staged: '?', Unstaged: '?'}}

	view := typing(t, unstaged.live(t, 120, 40), "3", "c").View()
	requireScreen(t, view, "nothing is staged: space stages the selected file")
	refuseScreen(t, view, "┏━ Commit")
}

func TestTheComposerAssemblesAConventionalCommit(t *testing.T) {
	t.Parallel()

	composing := newWorld()

	composer := typing(t, composing.live(t, 120, 40), "3", "c")
	requireScreen(t, composer.View(), "┏━ Commit", "‹feat›", "Refs: PROJ-412", "1 files staged",
		"no body yet: ctrl+e writes one in your editor")

	// Type, scope and subject, then the body from the editor.
	composing.edited = "Tokens reached the log."
	// The composer opens on the subject: back twice to the type, then forward.
	keys := []string{keyShiftTab, keyShiftTab, "right", keyTab}
	keys = append(append(keys, letters("config")...), keyTab)
	keys = append(append(keys, letters("redact tokens")...), "ctrl+e")

	filled := typing(t, composer, keys...)

	requireScreen(t, filled.View(), "‹fix›", "fix(config): redact tokens  26/72", "Tokens reached the log.")

	committed := typing(t, filled, keyEnter)
	requireScreen(t, committed.View(), "● committed fix(config): redact tokens")
	refuseScreen(t, committed.View(), "┏━ Commit", "┏━ git commit")

	want := "commit fix(config): redact tokens\n\nTokens reached the log.\n\nRefs: PROJ-412\n"
	if calls := composing.asked("commit"); len(calls) != 1 || calls[0] != want {
		t.Errorf("commit calls = %q, want %q", calls, want)
	}
}

func TestTheComposerRefusesASubjectThatBreaksTheRules(t *testing.T) {
	t.Parallel()

	composing := newWorld()

	long := typing(t, composing.live(t, 120, 40), append([]string{"3", "c"}, letters(strings.Repeat("x", 70))...)...)
	requireScreen(t, long.View(), "✗ the subject is too long: 76 of 72 characters")

	refused := typing(t, long, keyEnter)
	requireScreen(t, refused.View(), "┏━ Commit")

	empty := typing(t, composing.live(t, 120, 40), "3", "c", keyEnter)
	requireScreen(t, empty.View(), "✗ the subject needs a description")

	if calls := composing.asked("commit"); len(calls) != 0 {
		t.Errorf("committed a subject that breaks the rules: %q", calls)
	}
}

func TestTheTypeCyclesBothWays(t *testing.T) {
	t.Parallel()

	composer := typing(t, newWorld().live(t, 120, 40), "3", "c", keyTab)
	requireScreen(t, typing(t, composer, "left").View(), "‹revert›")
	requireScreen(t, typing(t, composer, "right", "right").View(), "‹docs›")

	// Arrows on a text field move its cursor, not the type.
	requireScreen(t, typing(t, composer, keyTab, "right").View(), "‹feat›")
}

func TestAFailedCommitShowsWhereToLookAndKeepsTheDraft(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{
		"┃  golangci-lint ❯ ",
		"internal/tui/pane.go:64:1: cyclomatic complexity 12 of func `Update` is high (> 10) (gocyclo)",
		"README.md:3:81 MD013/line-length Line length",
		"summary: (done in 2.1 seconds)",
		"✔️ gofmt (0.1 seconds)",
		"🥊 golangci-lint (2.0 seconds)",
	}

	failed := typing(t, failing.live(t, 120, 40), commitKeys("redact tokens")...)
	requireScreen(t, failed.View(), "┏━ git commit", "✗ exit status 1", "✗ golangci-lint · ● gofmt",
		"▸ internal/tui/pane.go:64 cyclomatic complexity", "README.md:3 MD013", "enter open in editor", "r run again")

	opened := typing(t, failed, "j", keyEnter)
	if calls := failing.asked("open-editor"); len(calls) != 1 || calls[0] != "open-editor README.md:3" {
		t.Errorf("editor calls = %q, want README.md at line 3", calls)
	}

	// Closing the run and composing again starts from the draft.
	reopened := typing(t, opened, "esc", "c")
	requireScreen(t, reopened.View(), "> redact tokens")

	// Running it again commits again.
	typing(t, failed, "r")

	if calls := failing.asked("commit"); len(calls) != 2 {
		t.Errorf("commit calls = %d, want the retry to commit again", len(calls))
	}
}

func TestAnEditorThatCannotOpenIsReported(t *testing.T) {
	t.Parallel()

	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"main.go:3:1: wrong"}
	failing.editorErr = errEditorFailed

	view := typing(t, failing.live(t, 120, 40), commitKeys("x", keyEnter)...).View()
	requireScreen(t, view, "✗ the editor exited with an error")
}

func TestARunningCommitCannotBeLeft(t *testing.T) {
	t.Parallel()

	composing := newWorld()

	composer := typing(t, composing.live(t, 120, 40), append([]string{"3", "c"}, letters("x")...)...)
	running, _ := pressed(t, composer, keyEnter)

	requireScreen(t, running.View(), "┏━ git commit", "◐ running…")
	requireScreen(t, footerLine(running.View()), "ctrl+c quit")
	requireScreen(t, press(t, running, "esc", "r").View(), "◐ running…")
}

func TestHRunsThePreCommitHookWithoutCommitting(t *testing.T) {
	t.Parallel()

	hooked := newWorld()
	hooked.commitLines = []string{"┃  lint ❯ ", "summary: (done in 1.0 seconds)", "✔️ lint (1.0 seconds)"}

	passed := typing(t, hooked.live(t, 120, 40), "3", "h")
	requireScreen(t, passed.View(), "┏━ pre-commit", "● done", "● lint", "r run again")

	if calls := hooked.asked("hook"); len(calls) != 1 || calls[0] != "hook pre-commit" {
		t.Errorf("hook calls = %q", calls)
	}

	if calls := hooked.asked("commit"); len(calls) != 0 {
		t.Errorf("running the hook committed: %q", calls)
	}

	requireScreen(t, typing(t, passed, "esc").View(), focused("3 Commits"))
}

func TestADryRunCommitsNothing(t *testing.T) {
	t.Parallel()

	dry := newWorld()

	model := sized(t, dryInterface(dry), 120, 40)
	model = drain(t, model, model.Init())

	committed := typing(t, model, commitKeys("x")...)
	requireScreen(t, committed.View(), "dry run: would commit feat: x")
	requireScreen(t, typing(t, model, "3", "h").View(), "dry run: would run the pre-commit hook")

	if calls := append(dry.asked("commit"), dry.asked("hook")...); len(calls) != 0 {
		t.Errorf("a dry run committed or ran a hook: %q", calls)
	}
}
