// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// wheel is one notch of the mouse wheel over a cell.
func wheel(t *testing.T, model tui.Model, column, row int, button tea.MouseButton) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseWheelMsg{X: column, Y: row, Button: button})

	return drain(t, concrete(t, updated), cmd)
}

// notches turns the wheel the same way count times over a cell.
func notches(t *testing.T, model tui.Model, count int, button tea.MouseButton) tui.Model {
	t.Helper()

	for range count {
		model = wheel(t, model, 80, 10, button)
	}

	return model
}

func TestTheWheelScrollsTheDetail(t *testing.T) {
	t.Parallel()

	// Arrange
	wordy := newWorld()
	wordy.detail.Description = strings.Repeat("line\n", 60) + "THE END"
	screen := wordy.live(t, 120, 30)

	// Act: wheel down over the detail
	down := notches(t, screen, 25, tea.MouseWheelDown)

	// Assert: the end is in sight
	requireScreen(t, down.View().Content, "THE END")
	refuseScreen(t, down.View().Content, detailTop)

	// Act: wheel back up
	raised := notches(t, down, 30, tea.MouseWheelUp)

	// Assert: the top is in sight
	requireScreen(t, raised.View().Content, detailTop)
	refuseScreen(t, raised.View().Content, "THE END")

	// Act: wheel over the rail
	overRail := wheel(t, raised, 5, 5, tea.MouseWheelDown)

	// Assert: nothing moves
	if overRail.View().Content != raised.View().Content {
		t.Errorf("the wheel over the rail changed the screen:\n%s", overRail.View().Content)
	}
}

func TestTheWheelMovesAnOverlaysList(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act: wheel down
	down := wheel(t, picker, 80, 10, tea.MouseWheelDown)

	// Assert: the selection moves down
	requireScreen(t, down.View().Content, "▸ ● Done")

	// Act: wheel up
	up := wheel(t, down, 80, 10, tea.MouseWheelUp)

	// Assert: and back up
	requireScreen(t, up.View().Content, "▸ ◐ Start Review")
}

func TestClickingPicksAnIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height, column int
	}{
		// The issue list in the focused rail pane: its second row is row 3.
		"in the rail": {width: 120, height: 40, column: 5},
		// Below 80 columns the rail is dropped and the list fills the detail.
		"on a narrow terminal": {width: 79, height: 30, column: 10},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			screen := newWorld().live(t, tt.width, tt.height)

			// Act
			view := click(t, screen, tt.column, 3).View().Content

			// Assert
			requireScreen(t, view, "▸ ○ PROJ-388")
		})
	}
}

func TestClickingPicksATransition(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act: click the second transition
	// Row 6: the border, the issue, its status and a blank line come first.
	second := click(t, picker, 60, 6)

	// Assert: it is selected
	requireScreen(t, second.View().Content, "▸ ● Done")

	// Act: click the first
	first := click(t, second, 60, 5)

	// Assert: it is selected
	requireScreen(t, first.View().Content, "▸ ◐ Start Review")

	// Act: click the issue above the transitions
	header := click(t, first, 60, 2)

	// Assert: nothing changes
	if header.View().Content != first.View().Content {
		t.Errorf("a click on the picker's header changed the screen:\n%s", header.View().Content)
	}
}

func TestAClickOutsideAnOpenOverlayDoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act
	clicked := click(t, picker, 5, 30)

	// Assert
	if clicked.View().Content != picker.View().Content {
		t.Errorf("a click outside the picker changed the screen:\n%s", clicked.View().Content)
	}
}

func TestClickingARunsFailuresPicksOne(t *testing.T) {
	t.Parallel()

	// Arrange
	failing := newWorld()
	failing.commitErr = errHookFailed
	failing.commitLines = []string{"a.go:1:1: first", "b.go:2:1: second"}
	failed := typing(t, failing.live(t, 120, 40), commitKeys("x")...)

	// Act: click the second failure
	// The border, the outcome and a blank line come before the list.
	picked := click(t, failed, 60, 5)

	// Assert: it is selected
	requireScreen(t, picked.View().Content, "▸ b.go:2 second")

	// Act: click above the list
	above := click(t, picked, 60, 1)

	// Assert: nothing changes
	if above.View().Content != picked.View().Content {
		t.Errorf("a click above the failures changed the screen:\n%s", above.View().Content)
	}
}

func TestTheMouseSettingDecidesWhetherItIsCaptured(t *testing.T) {
	t.Parallel()

	// m flips capture from wherever the setting started it, and says what it did.
	cases := map[string]struct {
		mouse  bool
		want   tea.MouseMode
		notice string
	}{
		"captured":     {mouse: true, want: tea.MouseModeNone, notice: "mouse off"},
		"not captured": {mouse: false, want: tea.MouseModeCellMotion, notice: "mouse on"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := completeConfig()
			cfg.UI.Mouse = tt.mouse
			model := sized(t, tui.New(cfg, nil, tui.Deps{}), 120, 40)

			// Act
			updated, _ := model.Update(keyMsg("m"))
			after := concrete(t, updated)

			// Assert
			// v2 sets the mouse mode declaratively in View, so the toggle shows in
			// the next View's MouseMode rather than in a returned command.
			if got := after.View().MouseMode; got != tt.want {
				t.Errorf("m left mouse mode %v, want %v", got, tt.want)
			}

			requireScreen(t, after.View().Content, tt.notice)
		})
	}
}
