// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/tui"
)

// pasting delivers a bracketed paste, as the terminal sends one, and runs what
// it leads to.
func pasting(t *testing.T, model tui.Model, content string) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.PasteMsg{Content: content})

	return drain(t, concrete(t, updated), cmd)
}

func TestAPasteIsTypedIntoTheFieldThatHasTheKeyboard(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newWorld()
	form := typing(t, repo.live(t, 120, 40), "a")

	// Act: paste a username into the assign form
	pasted := pasting(t, form, "fred")

	// Assert: nothing is sent before it is confirmed
	if got := repo.asked("assign "); len(got) != 0 {
		t.Errorf("assigned on a paste: %v", got)
	}

	// Act: confirm
	confirmed := typing(t, pasted, keyEnter)

	// Assert: the pasted username is the one sent
	if got := repo.asked("assign " + issueKey + " fred"); len(got) != 1 {
		t.Errorf("assign calls = %v, want the pasted username sent; screen:\n%s", got, confirmed.View().Content)
	}
}

func TestAPasteNarrowsTheIssuesFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	filtering := typing(t, newWorld().live(t, 120, 40), "/")

	// Act
	view := pasting(t, filtering, "388").View().Content

	// Assert
	requireScreen(t, view, secondIssue)
	refuseScreen(t, view, issueKey+" "+statusInProgress)
}

func TestAPastedLineBreakSeparatesWordsInAOneLineField(t *testing.T) {
	t.Parallel()

	// Arrange
	// Terminals often send a pasted line break as a carriage return, which a
	// sanitizer that drops control characters would otherwise glue shut.
	repo := newWorld()
	form := typing(t, repo.live(t, 120, 40), "w")

	// Act
	pasted := pasting(t, form, "2h\r\x1b[2Jdone")

	// Assert
	if view := pasted.View().Content; !strings.Contains(view, "2h done") {
		t.Errorf("the pasted text was not typed as \"2h done\":\n%s", view)
	}
}

func TestAPasteWithNowhereToGoChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newWorld().live(t, 120, 40)
	before := model.View().Content

	// Act
	after := pasting(t, model, "q").View().Content

	// Assert
	if after != before {
		t.Errorf("a paste with no text field open changed the screen:\n%s", after)
	}
}

func TestAPasteFiltersTheOwnerPicker(t *testing.T) {
	t.Parallel()

	// Arrange
	picking := typing(t, onlyBen().live(t, 140, 40), "5", "p", "a")

	// Act
	view := pasting(t, picking, "qq\r\nzz").View().Content

	// Assert
	requireScreen(t, view, "filter  qq zz ", "Not on Slack")
	refuseScreen(t, view, carlaName)
}

// resolvingWorld is a world whose one transition asks for a resolution, then
// a root cause typed as text.
func resolvingWorld() *world {
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}

	return resolving
}

// versionsRefused is the keys that fill fieldfulMove's user and date, then
// confirm its list of versions with none chosen, which it refuses.
func versionsRefused() []string {
	keys := append(append([]string{"t", keyEnter}, letters("fred")...), keyEnter)

	return append(append(keys, letters("2026-09-21")...), keyEnter, keyEnter)
}

// pasteTarget is a form open on a field that takes text: the world it opens
// in, the keys that open it and type into the field, and what the field reads
// once a paste of "x" is typed after them.
type pasteTarget struct {
	world   func() *world
	keys    []string
	pastedX string
}

// pasteTargets are the forms whose field takes a paste, each with what enter
// sends still unsent.
func pasteTargets() map[string]pasteTarget {
	return map[string]pasteTarget{
		"the branch creator's name": {
			world: newWorld, keys: []string{"2", "b"}, pastedX: "> fix/PROJ-412-fix-token-redactionx",
		},
		"the pull request composer's title": {
			world: withoutPull, keys: []string{"4", "n"}, pastedX: "> " + pullTitle + "x",
		},
		"the pull request editor's title": {
			world: newWorld, keys: []string{"4", "e"}, pastedX: "> " + pullTitle + "x",
		},
		"a transition's text field": {
			world: resolvingWorld, keys: append([]string{"t", keyEnter, keyEnter}, letters("nil")...),
			pastedX: "> nilx",
		},
		"the task line": {
			world: withTasks, keys: append([]string{tasksPane, "a"}, letters("Water")...), pastedX: "> Waterx",
		},
		"the go-to prompt": {
			world: reposWorld, keys: append([]string{reposKey, "g"}, letters("~/src/web")...),
			pastedX: "> ~/src/webx",
		},
	}
}

func TestAPasteIsTypedIntoTheFieldEachFormHasOpen(t *testing.T) {
	t.Parallel()

	for name, target := range pasteTargets() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			opened := typing(t, target.world().live(t, 120, 40), target.keys...)

			// Act
			view := pasting(t, opened, "x").View().Content

			// Assert
			requireScreen(t, view, target.pastedX)
		})
	}
}

func TestAPasteWaitsUntilTheFormIsAnswered(t *testing.T) {
	t.Parallel()

	for name, target := range pasteTargets() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// Enter sends what the form asks for, a write or a look at the path
			// typed, and its command is left unanswered, as a slow service
			// leaves it.
			opened := typing(t, target.world().live(t, 120, 40), target.keys...)
			sending, _ := pressed(t, opened, keyEnter)
			before := sending.View().Content

			// Act
			view := pasting(t, sending, "x").View().Content

			// Assert
			refuseScreen(t, view, target.pastedX)

			if view != before {
				t.Errorf("a paste before the form was answered changed it:\n%s", plain(view))
			}
		})
	}
}

func TestAPasteWhereNoTextIsAskedForChangesNothing(t *testing.T) {
	t.Parallel()

	// Where a field shows why it was refused, the paste leaves that shown: it
	// typed nothing there that could answer it.
	cases := map[string]struct {
		moves []jira.Transition
		keys  []string
	}{
		"the transitions listed": {moves: []jira.Transition{resolveIssue()}, keys: []string{"t"}},
		"a transition's options, refusing": {
			moves: []jira.Transition{fieldfulMove()}, keys: versionsRefused(),
		},
		"the commit type, refusing": {keys: []string{"3", "c", keyEnter, keyShiftTab, keyShiftTab}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			repo := newWorld()
			repo.moves = tt.moves
			opened := typing(t, repo.live(t, 120, 40), tt.keys...)
			before := opened.View().Content

			// Act
			after := pasting(t, opened, "x").View().Content

			// Assert
			if after != before {
				t.Errorf("a paste with no text asked for changed the screen from\n%s\nto\n%s", plain(before), plain(after))
			}
		})
	}
}

func TestAPasteIsTypedIntoTheCommitsScopeOrSubject(t *testing.T) {
	t.Parallel()

	// The subject has the keyboard as the composer opens; shift+tab moves it
	// to the scope.
	cases := map[string]struct {
		keys []string
		want string
	}{
		"the subject": {keys: []string{"3", "c"}, want: "▸ subject > tidy"},
		"the scope":   {keys: []string{"3", "c", keyShiftTab}, want: "▸ scope   > tidy"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			opened := typing(t, newWorld().live(t, 120, 40), tt.keys...)

			// Act
			view := pasting(t, opened, "tidy").View().Content

			// Assert
			requireScreen(t, view, tt.want)
		})
	}
}
