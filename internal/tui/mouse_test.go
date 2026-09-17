// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/tui"
)

// wheel is one notch of the mouse wheel over a cell.
func wheel(t *testing.T, model tui.Model, column, row int, button tea.MouseButton) tui.Model {
	t.Helper()

	updated, cmd := model.Update(tea.MouseMsg{X: column, Y: row, Action: tea.MouseActionPress, Button: button})

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
	down := notches(t, screen, 25, tea.MouseButtonWheelDown)

	// Assert: the end is in sight
	requireScreen(t, down.View(), "THE END")
	refuseScreen(t, down.View(), detailTop)

	// Act: wheel back up
	raised := notches(t, down, 30, tea.MouseButtonWheelUp)

	// Assert: the top is in sight
	requireScreen(t, raised.View(), detailTop)
	refuseScreen(t, raised.View(), "THE END")

	// Act: wheel over the rail
	overRail := wheel(t, raised, 5, 5, tea.MouseButtonWheelDown)

	// Assert: nothing moves
	if overRail.View() != raised.View() {
		t.Errorf("the wheel over the rail changed the screen:\n%s", overRail.View())
	}
}

func TestTheWheelMovesAnOverlaysList(t *testing.T) {
	t.Parallel()

	// Arrange
	choosing := newWorld()
	choosing.moves = workflowMoves()
	picker := typing(t, choosing.live(t, 120, 40), "t")

	// Act: wheel down
	down := wheel(t, picker, 80, 10, tea.MouseButtonWheelDown)

	// Assert: the selection moves down
	requireScreen(t, down.View(), "▸ ● Done")

	// Act: wheel up
	up := wheel(t, down, 80, 10, tea.MouseButtonWheelUp)

	// Assert: and back up
	requireScreen(t, up.View(), "▸ ◐ Start Review")
}

func TestClickingPicksAnIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height, column int
	}{
		// The issue list in the focused rail pane: its second row is row 3.
		"in the rail": {width: 120, height: 40, column: 5},
		// On a narrow terminal the list is the detail.
		"on a narrow terminal": {width: 80, height: 30, column: 10},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			screen := newWorld().live(t, tt.width, tt.height)

			// Act
			view := click(t, screen, tt.column, 3).View()

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
	requireScreen(t, second.View(), "▸ ● Done")

	// Act: click the first
	first := click(t, second, 60, 5)

	// Assert: it is selected
	requireScreen(t, first.View(), "▸ ◐ Start Review")

	// Act: click the issue above the transitions
	header := click(t, first, 60, 2)

	// Assert: nothing changes
	if header.View() != first.View() {
		t.Errorf("a click on the picker's header changed the screen:\n%s", header.View())
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
	if clicked.View() != picker.View() {
		t.Errorf("a click outside the picker changed the screen:\n%s", clicked.View())
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
	requireScreen(t, picked.View(), "▸ b.go:2 second")

	// Act: click above the list
	above := click(t, picked, 60, 1)

	// Assert: nothing changes
	if above.View() != picked.View() {
		t.Errorf("a click above the failures changed the screen:\n%s", above.View())
	}
}

func TestTheMouseSettingDecidesWhetherItIsCaptured(t *testing.T) {
	t.Parallel()

	// m flips capture from wherever the setting started it.
	cases := map[string]struct {
		mouse bool
		want  string
	}{
		"captured":     {mouse: true, want: messageType(tea.DisableMouse)},
		"not captured": {mouse: false, want: messageType(tea.EnableMouseCellMotion)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := completeConfig()
			cfg.UI.Mouse = tt.mouse

			// Act
			_, cmd := tui.New(cfg, nil, tui.Deps{}).Update(keyMsg("m"))

			// Assert
			if got := messageType(cmd); got != tt.want {
				t.Errorf("m returned %s, want %s", got, tt.want)
			}
		})
	}
}
