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

// errHookFailed is how git reports a hook that failed.
var errHookFailed = errors.New("exit status 1")

func TestCommitNeedsSomethingStaged(t *testing.T) {
	t.Parallel()

	// Arrange
	unstaged := newWorld()
	unstaged.changes = []gitrepo.Change{{Path: "notes.txt", Staged: '?', Unstaged: '?'}}

	// Act
	view := typing(t, unstaged.live(t, 120, 40), "3", "c").View()

	// Assert
	requireScreen(t, view, "nothing is staged: space stages the selected file")
	refuseScreen(t, view, "┏━ Commit")
}

func TestTheComposerAssemblesAConventionalCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	composing.edited = "Tokens reached the log."
	model := composing.live(t, 120, 40)

	// The composer opens on the subject: back twice to the type, then forward.
	keys := []string{keyShiftTab, keyShiftTab, keyRight, keyTab}
	keys = append(append(keys, letters("config")...), keyTab)
	keys = append(append(keys, letters("redact tokens")...), "ctrl+o")

	// Act: open the composer
	composer := typing(t, model, "3", "c")

	// Assert: it starts from the issue and what is staged
	requireScreen(t, composer.View(), "┏━ Commit", "‹feat›", "Refs: PROJ-412", "1 file staged",
		"no body yet: ctrl+o writes one in your editor")

	// Act: choose the type, scope and subject, then write the body in the editor
	filled := typing(t, composer, keys...)

	// Assert: the message shows as it will be committed
	requireScreen(t, filled.View(), "‹fix›", "fix(config): redact tokens  26/72", "Tokens reached the log.")

	// Act: commit
	committed := typing(t, filled, keyEnter)

	// Assert: git committed exactly that message, and both overlays closed
	requireScreen(t, committed.View(), "● committed fix(config): redact tokens")
	refuseScreen(t, committed.View(), "┏━ Commit", "┏━ git commit")

	want := "commit fix(config): redact tokens\n\nTokens reached the log.\n\nRefs: PROJ-412\n"
	if calls := composing.asked("commit"); len(calls) != 1 || calls[0] != want {
		t.Errorf("commit calls = %q, want %q", calls, want)
	}
}

func TestTheComposerRefusesATooLongSubject(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	model := composing.live(t, 120, 40)

	// Act: type a subject past the limit
	long := typing(t, model, append([]string{"3", "c"}, letters(strings.Repeat("x", 70))...)...)

	// Assert: the problem is named
	requireScreen(t, long.View(), "✗ the subject is too long: 76 of 72 characters")

	// Act: try to commit it
	refused := typing(t, long, keyEnter)

	// Assert: nothing is committed, and the composer stays open
	requireScreen(t, refused.View(), "┏━ Commit")

	if calls := composing.asked("commit"); len(calls) != 0 {
		t.Errorf("committed a subject that is too long: %q", calls)
	}
}

func TestTheComposerRefusesAnEmptySubject(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()

	// Act
	empty := typing(t, composing.live(t, 120, 40), "3", "c", keyEnter)

	// Assert
	requireScreen(t, empty.View(), "✗ the subject needs a description")

	if calls := composing.asked("commit"); len(calls) != 0 {
		t.Errorf("committed an empty subject: %q", calls)
	}
}

func TestTheTypeCyclesBothWays(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"left wraps back to the last type": {keys: []string{"left"}, want: "‹revert›"},
		"right moves forward":              {keys: []string{keyRight, keyRight}, want: "‹docs›"},
		// Arrows on a text field move its cursor, not the type.
		"arrows on a text field": {keys: []string{keyTab, keyRight}, want: "‹feat›"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			composer := typing(t, newWorld().live(t, 120, 40), "3", "c", keyTab)

			// Act
			view := typing(t, composer, tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}

func TestAFailedRunLeadsWithTheStepAndShowsFullOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{
		"main.go:3:1: undefined: go",
		".git/hooks/pre-commit: line 3: go: command not found",
	}
	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// Act & Assert: the headline names the step, not git's exit code
	requireScreen(t, failed.View(), "the commit was refused")

	// Act: switch to the full output
	full := typing(t, failed, "o")

	// Assert: the whole output can be read
	requireScreen(t, full.View(), "go: command not found")
}

func TestAFailedCommitShowsWhereToLookAndKeepsTheDraft(t *testing.T) {
	t.Parallel()

	// Arrange
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
	model := failing.live(t, 120, 40)

	// Act: commit, and have the hook fail
	failed := typing(t, model, commitKeys("redact tokens")...)

	// Assert: the run says which jobs failed and where they point
	requireScreen(t, failed.View(), "┏━ git commit", "✗ the commit was refused", "✗ golangci-lint · ● gofmt",
		"▸ internal/tui/pane.go:64 cyclomatic complexity", "README.md:3 MD013", "enter open in editor", "r run again")

	// Act: open the second place in the editor
	opened := typing(t, failed, "j", keyEnter)

	// Assert: the editor opens that file at that line
	if calls := failing.asked("open-editor"); len(calls) != 1 || calls[0] != "open-editor README.md:3" {
		t.Errorf("editor calls = %q, want README.md at line 3", calls)
	}

	// Act: close the run and compose again
	reopened := typing(t, opened, keyEsc, "c")

	// Assert: the composer starts from the draft
	requireScreen(t, reopened.View(), "> redact tokens")
}

func TestAFailedCommitCanBeRunAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"main.go:3:1: wrong"}
	failed := typing(t, failing.live(t, 120, 40), commitKeys("redact tokens")...)

	// Act
	typing(t, failed, "r")

	// Assert
	if calls := failing.asked("commit"); len(calls) != 2 || calls[1] != calls[0] {
		t.Errorf("commit calls = %q, want the same commit tried again", calls)
	}
}

func TestAnEditorThatCannotOpenIsReported(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"main.go:3:1: wrong"}
	failing.editorErr = errEditorFailed

	// Act
	view := typing(t, failing.live(t, 120, 40), commitKeys("x", keyEnter)...).View()

	// Assert
	requireScreen(t, view, "✗ the editor exited with an error")
}

func TestARunningCommitCannotBeLeft(t *testing.T) {
	t.Parallel()

	for _, key := range []string{keyEsc, "r"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			// Arrange
			composer := typing(t, newWorld().live(t, 120, 40), append([]string{"3", "c"}, letters("x")...)...)
			running, _ := pressed(t, composer, keyEnter)

			// Act
			after, cmd := pressed(t, running, key)

			// Assert
			if cmd != nil {
				t.Errorf("%s produced a command while the commit runs", key)
			}

			if after.View() != running.View() {
				t.Errorf("%s changed the screen while the commit runs:\n%s", key, after.View())
			}
		})
	}
}

func TestARunningCommitOffersOnlyQuitting(t *testing.T) {
	t.Parallel()

	// Arrange
	composer := typing(t, newWorld().live(t, 120, 40), append([]string{"3", "c"}, letters("x")...)...)

	// Act
	running, _ := pressed(t, composer, keyEnter)

	// Assert
	requireScreen(t, running.View(), "┏━ git commit", "◐ running…")
	requireScreen(t, footerLine(running.View()), "ctrl+c quit")
	refuseScreen(t, footerLine(running.View()), keyEsc, "r run again")
}

func TestHRunsThePreCommitHookWithoutCommitting(t *testing.T) {
	t.Parallel()

	// Arrange
	hooked := newWorld()
	hooked.commitLines = []string{"┃  lint ❯ ", "summary: (done in 1.0 seconds)", "✔️ lint (1.0 seconds)"}
	model := hooked.live(t, 120, 40)

	// Act: run the hook
	passed := typing(t, model, "3", "h")

	// Assert: it ran, alone, and says how it went
	requireScreen(t, passed.View(), "┏━ pre-commit", "● done", "● lint", "r run again")

	if calls := hooked.asked("hook"); len(calls) != 1 || calls[0] != "hook pre-commit" {
		t.Errorf("hook calls = %q", calls)
	}

	if calls := hooked.asked("commit"); len(calls) != 0 {
		t.Errorf("running the hook committed: %q", calls)
	}

	// Act: close the run
	closed := typing(t, passed, keyEsc).View()

	// Assert: the keyboard is back on the Commits pane
	refuseScreen(t, closed, "┏━ pre-commit")
	requireScreen(t, closed, focused("3 Commits"))
}

func TestADryRunCommitsNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		keys []string
		want string
	}{
		"committing":       {keys: commitKeys("x"), want: "dry run: would commit feat: x"},
		"running the hook": {keys: []string{"3", "h"}, want: "dry run: would run the pre-commit hook"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dry := newWorld()
			model := sized(t, dryInterface(dry), 120, 40)
			model = drain(t, model, model.Init())

			// Act
			view := typing(t, model, tt.keys...).View()

			// Assert
			requireScreen(t, view, tt.want)

			if calls := append(dry.asked("commit"), dry.asked("hook")...); len(calls) != 0 {
				t.Errorf("a dry run committed or ran a hook: %q", calls)
			}
		})
	}
}

// failingLint is a world whose commit fails a lint job with two places to look.
func failingLint() *world {
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{
		"┃  lint ❯ ", "a.go:1:1: first", "b.go:2:1: second",
		"summary: (done in 1 seconds)", "🥊 lint (1 seconds)",
	}

	return failing
}

func TestARunsFailuresMoveBothWays(t *testing.T) {
	t.Parallel()

	// Arrange
	failed := typing(t, failingLint().live(t, 120, 40), commitKeys("x")...)

	// Act: j
	down := typing(t, failed, "j")

	// Assert: the second failure is selected
	requireScreen(t, down.View(), "▸ b.go:2 second")

	// Act: k
	up := typing(t, down, "k")

	// Assert: the first is selected again
	requireScreen(t, up.View(), "▸ a.go:1 first")
}

func TestARunsFailuresListedBelowItsJobsCanBeClicked(t *testing.T) {
	t.Parallel()

	// Arrange
	failed := typing(t, failingLint().live(t, 120, 40), commitKeys("x")...)

	// Act
	// With jobs listed above them, the list starts a row lower.
	picked := click(t, failed, 60, 6)

	// Assert
	requireScreen(t, picked.View(), "▸ b.go:2 second")
}

func TestARunsFailureNeedsAnEditorToOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	noEditor := failingLint().deps()
	noEditor.Editor.Open = nil
	model := sized(t, tui.New(completeConfig(), nil, noEditor), 120, 40)
	failed := typing(t, drain(t, model, model.Init()), commitKeys("x")...)

	// Act
	after, cmd := pressed(t, failed, keyEnter)

	// Assert
	if cmd != nil || after.View() != failed.View() {
		t.Errorf("enter did something with no editor:\n%s", after.View())
	}
}

func TestTheComposerCountsOneFileInTheSingular(t *testing.T) {
	t.Parallel()

	// Arrange
	committing := newWorld()

	// Act
	composer := typing(t, committing.live(t, 120, 40), "3", "c")

	// Assert
	requireScreen(t, composer.View(), "1 file staged")
	refuseScreen(t, composer.View(), "1 files staged")
}

func TestTheComposerFlagsAnInvalidScopeAsItIsTyped(t *testing.T) {
	t.Parallel()

	// Arrange
	composing := newWorld()
	opened := typing(t, composing.live(t, 120, 40), "3", "c", keyShiftTab)

	// Act
	view := typing(t, opened, letters("BAD")...).View()

	// Assert
	lines := strings.Split(view, "\n")
	scopeRow := -1

	for index, line := range lines {
		if strings.Contains(line, "scope") {
			scopeRow = index

			break
		}
	}

	if scopeRow < 0 || scopeRow+1 >= len(lines) || !strings.Contains(lines[scopeRow+1], "a scope is lowercase letters") {
		t.Errorf("the scope error is not shown under the scope field:\n%s", view)
	}
}

func TestCtrlEIsLeftToTheSubjectField(t *testing.T) {
	t.Parallel()

	// Arrange
	editing := newWorld()
	composer := typing(t, editing.live(t, 120, 40), "3", "c")

	// Act
	typing(t, composer, append(letters("redact tokens"), "ctrl+e")...)

	// Assert
	if calls := editing.asked("edit"); len(calls) != 0 {
		t.Errorf("ctrl+e opened the editor: %q", calls)
	}
}
