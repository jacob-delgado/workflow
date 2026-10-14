// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/tui/frame"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// floating is an overlay drawn as a dialog over the screen rather than in
// the detail pane: the panes stay in view behind it, faint.
type floating interface {
	// wants is the size the dialog would like, its frame included, when it
	// may be at most most.
	wants(kit renderKit, most layout.Size) layout.Size
}

// dialogFrame is the columns and rows a framed dialog spends on its frame and
// the space inside it.
const (
	dialogFrameColumns = detailPadding
	dialogFrameRows    = 2
)

// dialogBox is where the open overlay is drawn as a dialog, and whether it
// floats at all.
func (m Model) dialogBox() (layout.Box, bool) {
	floater, floats := m.overlay.(floating)
	if !floats {
		return layout.Box{}, false
	}

	shape := m.shape()
	most := layout.Size{Width: shape.Footer.Width - dialogFrameColumns, Height: shape.Detail.Height - dialogFrameRows}

	return shape.Dialog(floater.wants(m.kit(), most)), true
}

// sheet reports whether a dialog in box is drawn as a sheet over the whole
// body, with no frame to spend columns on, as the detail pane drops its own.
func sheet(box layout.Box) bool {
	return box.Width < layout.BorderlessBelow
}

// dialogInside is the columns and rows of content a dialog in box holds.
func dialogInside(box layout.Box) (int, int) {
	if sheet(box) {
		return box.Width, max(0, box.Height-1)
	}

	return max(1, box.Width-dialogFrameColumns), frame.BodyRows(box.Height)
}

// withDialog is the body of the screen — the rail and the detail — faint
// behind the open overlay drawn as a dialog in box over it.
func (m Model) withDialog(shape layout.Layout, box layout.Box) string {
	behind := m
	behind.overlay = nil

	width, rows := dialogInside(box)
	title, content, style := m.overlayContent(width, rows)

	drawn := frame.Render(title, content, box.Width, box.Height, style)
	if sheet(box) {
		drawn = frame.Plain(title, content, box.Width, box.Height, style)
	}

	return lipgloss.NewCompositor(
		lipgloss.NewLayer(m.backdrop(behind.body(shape))),
		lipgloss.NewLayer(drawn).X(box.X).Y(box.Y-shape.Detail.Y).Z(1),
	).Render()
}

// backdrop is the body drawn faint, its own colors and weights given up so
// nothing behind a dialog competes with it.
func (m Model) backdrop(body string) string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		lines[index] = m.styles.label.Render(ansi.Strip(line))
	}

	return strings.Join(lines, "\n")
}
