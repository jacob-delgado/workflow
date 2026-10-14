// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// On a fake clock, a key typed into a form's text field is seen to wait on
// nothing: a cursor's blink would wait half a second of it before arriving.
func TestTypingIntoAFormsTextFieldWaitsOnNoTimer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		resolving := newWorld()
		resolving.moves = []jira.Transition{resolveIssue()}
		cause := typing(t, resolving.live(t, 120, 40), "t", keyEnter, keyEnter)
		before := time.Now()

		// Act
		typed := typing(t, cause, "x")

		// Assert
		if waited := time.Since(before); waited != 0 {
			t.Errorf("typing a letter into the root cause waited %v, want no wait at all", waited)
		}

		requireScreen(t, typed.View().Content, "> x")
	})
}

func TestAFormsTextFieldIsDrawnInTheInterfacesStyles(t *testing.T) {
	t.Parallel()

	// Arrange
	moving := newWorld()
	moving.moves = []jira.Transition{fieldfulMove()}
	user := typing(t, moving.live(t, 120, 40), "t", keyEnter)

	// Act
	dated := typing(t, user, append(letters("fred"), keyEnter)...)

	// Assert
	view := dated.View().Content
	if title := opening(lipgloss.NewStyle().Bold(true)) + "Due date (2 of 3)"; !strings.Contains(view, title) {
		t.Errorf("the date field's title is not strong, as a form's titles are: %q", view)
	}

	// The cursor stands on the shape's first letter, so the rest is what the
	// placeholder's style draws.
	if shape := opening(lipgloss.NewStyle().Faint(true)) + "YYY-MM-DD"; !strings.Contains(view, shape) {
		t.Errorf("the shape a date takes is not faint, as a hint is everywhere else: %q", view)
	}
}

func TestPreviousFieldMovedInUIKeysGoesBackFromATextField(t *testing.T) {
	t.Parallel()

	// Arrange
	resolving := newWorld()
	resolving.moves = []jira.Transition{resolveIssue()}
	resolving.cfg.UI.Keys = map[string]string{"previous-field": "f5"}
	// Huh leaves a field only once what it holds passes its check, so the root
	// cause holds something before going back from it.
	cause := typing(t, resolving.live(t, 120, 40), "t", keyEnter, keyEnter, "x")

	// Act
	resolution := typing(t, cause, "f5")

	// Assert
	requireScreen(t, resolution.View().Content, "Resolution (1 of 2)")
	refuseScreen(t, resolution.View().Content, "Root cause (2 of 2)")
}
