// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
