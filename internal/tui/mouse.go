// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// wheelLines is how far one notch of the wheel scrolls the detail.
const wheelLines = 3

// handleMouse answers a click or the wheel.
func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	shape := m.shape()

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
		return m.click(shape, msg.X, msg.Y)
	case msg.Button == tea.MouseButtonWheelDown:
		return m.wheel(shape, msg.X, msg.Y, 1)
	case msg.Button == tea.MouseButtonWheelUp:
		return m.wheel(shape, msg.X, msg.Y, -1)
	default:
		return m, nil
	}
}

// click focuses the rail pane under the pointer, picks the row clicked in a pane
// that already has focus, and picks a row in the detail. An open overlay holds on
// to the keyboard, so a click only ever picks within it.
func (m Model) click(shape layout.Layout, column, row int) (Model, tea.Cmd) {
	line := row - shape.Detail.Y - 1

	if m.overlay != nil {
		chooser, picks := m.overlay.(clickable)
		if !picks || !shape.Detail.Contains(column, row) {
			return m, nil
		}

		return chooser.click(m, line)
	}

	index, inRail := shape.RailAt(column, row)

	switch {
	case inRail && pane(index) != m.focus:
		return m.focusOn(pane(index)), nil
	case inRail:
		box := shape.Rail[index]

		// A rail pane's box holds one shared-rule row above its content, so skip
		// that row and count the rest as content.
		return m.pick(row-box.Y-1, max(0, box.Height-1), true)
	case shape.Detail.Contains(column, row):
		return m.pick(line, m.detailRows(), false)
	default:
		return m, nil
	}
}

// pick hands a clicked line to the focused pane, if it has a list.
func (m Model) pick(line, rows int, inRail bool) (Model, tea.Cmd) {
	picker := behaviorOf(m.focus).pick
	if picker == nil {
		return m, nil
	}

	return picker(m, line, rows, inRail)
}

// wheel scrolls: an open overlay's list, or the detail pane.
func (m Model) wheel(shape layout.Layout, column, row, step int) (Model, tea.Cmd) {
	if !shape.Detail.Contains(column, row) {
		return m, nil
	}

	if m.overlay != nil {
		direction := tea.KeyMsg{Type: tea.KeyDown}
		if step < 0 {
			direction = tea.KeyMsg{Type: tea.KeyUp}
		}

		return m.overlay.handleKey(m, direction)
	}

	return m.scrollDetail(step * wheelLines), nil
}
