// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// errScopeHistory stands in for a git failure to read recent commit subjects.
var errScopeHistory = errors.New("git could not read the log")

// scopeKeys opens the commit composer, moves back to the scope field, types
// prefix, then presses tab.
func scopeKeys(prefix string) []string {
	return append(append([]string{"3", "c", keyShiftTab}, letters(prefix)...), keyTab)
}

// staged makes each path a staged modification.
func staged(paths ...string) []gitrepo.Change {
	changes := make([]gitrepo.Change, 0, len(paths))
	for _, path := range paths {
		changes = append(changes, gitrepo.Change{Path: path, Staged: 'M', Unstaged: ' '})
	}

	return changes
}

func TestTheScopeCompletesToTheStagedFilesDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.changes = staged("internal/tui/a.go", "internal/tui/b.go")

	// Act
	view := typing(t, world.live(t, 120, 40), scopeKeys("t")...).View().Content

	// Assert
	requireScreen(t, view, "scope   > tui")
}

func TestTheScopeCompletesToAScopeAlreadyInTheLog(t *testing.T) {
	t.Parallel()

	// Arrange
	world := newWorld()
	world.recentSubjects = []string{"feat(jira): add a field", "fix(hooks): tidy"}

	// Act
	view := typing(t, world.live(t, 120, 40), scopeKeys("ji")...).View().Content

	// Assert
	requireScreen(t, view, "scope   > jira")
}

func TestTheScopeCompletesToTheSharedParentAcrossDirectories(t *testing.T) {
	t.Parallel()

	// Arrange
	// Files staged across two directories share only their parent, so that is the
	// scope offered.
	world := newWorld()
	world.changes = staged("internal/tui/a.go", "internal/jira/b.go")

	// Act
	view := typing(t, world.live(t, 120, 40), scopeKeys("in")...).View().Content

	// Assert
	requireScreen(t, view, "scope   > internal")
}

func TestTheScopeCountsOnlyTheStagedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	// The unstaged file under web/ would drag the shared directory to the root;
	// only the staged file counts, so the scope is still its directory.
	world := newWorld()
	world.changes = []gitrepo.Change{
		{Path: "internal/tui/a.go", Staged: 'M', Unstaged: ' '},
		{Path: "web/app.ts", Staged: ' ', Unstaged: 'M'},
	}

	// Act
	view := typing(t, world.live(t, 120, 40), scopeKeys("t")...).View().Content

	// Assert
	requireScreen(t, view, "scope   > tui")
}

func TestTheScopeSkipsADirectoryNameThatIsNotAScope(t *testing.T) {
	t.Parallel()

	// Arrange
	// The shared directory's name holds a space, which a scope may not, so it is
	// not offered and there is nothing to complete.
	world := newWorld()
	world.changes = staged("my dir/a.go", "my dir/b.go")
	world.noRecentSubjects = true

	keys := append(scopeKeys("m"), letters("Q")...)

	// Act
	view := typing(t, world.live(t, 120, 40), keys...).View().Content

	// Assert
	// The directory name is never offered, so tab navigated and its letters
	// never reached the scope.
	requireScreen(t, view, "scope   > m")
	refuseScreen(t, view, "dir")
}

func TestTabNavigatesPastTheScopeWhenNothingCompletes(t *testing.T) {
	t.Parallel()

	// Arrange
	// A scope is suggested, but the typed prefix matches none of it, so the tab
	// must move on: a letter after it lands in the subject, not the scope.
	world := newWorld()
	world.changes = staged("internal/tui/a.go")

	keys := append(scopeKeys("zzz"), letters("Q")...)

	// Act
	view := typing(t, world.live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, "scope   > zzz")
	refuseScreen(t, view, "zzzQ")
}

func TestTheScopeStaysPlainWithNothingToSuggest(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file staged at the repository root shares no directory, and the history
	// seam is absent, so there is nothing to complete and tab navigates.
	world := newWorld()
	world.changes = staged("main.go")
	world.noRecentSubjects = true

	keys := append(scopeKeys("x"), letters("Q")...)

	// Act
	view := typing(t, world.live(t, 120, 40), keys...).View().Content

	// Assert
	requireScreen(t, view, "scope   > x")
	refuseScreen(t, view, "xQ")
}

func TestTheDirectoryStillCompletesWhenTheHistoryFails(t *testing.T) {
	t.Parallel()

	// Arrange
	// The log source errors, but the staged directory still completes the scope,
	// so one source failing does not disable the other.
	world := newWorld()
	world.changes = staged("internal/tui/a.go")
	world.recentSubjectsErr = errScopeHistory

	// Act
	view := typing(t, world.live(t, 120, 40), scopeKeys("t")...).View().Content

	// Assert
	requireScreen(t, view, "scope   > tui")
}
