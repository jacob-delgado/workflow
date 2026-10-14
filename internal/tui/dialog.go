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
	// wants is the size the dialog would like, its frame included, when what
	// it holds inside its frame may be at most most.
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
	room := shape.DialogRoom()
	most := layout.Size{Width: room.Width - dialogFrameColumns, Height: room.Height - dialogFrameRows}

	return shape.Dialog(floater.wants(m.kit(), most)), true
}

// dialogColumns is the columns inside a dialog that would like width
// columns, its frame included, on this screen.
func (m Model) dialogColumns(width int) int {
	shape := m.shape()
	columns, _ := dialogInside(shape, shape.Dialog(layout.Size{Width: width, Height: 1}))

	return columns
}

// dialogInside is the columns and rows of content a dialog in box holds on
// a screen of shape: all of a sheet's but its title row, which has no frame
// to spend columns on, and a floating dialog's inside its frame.
func dialogInside(shape layout.Layout, box layout.Box) (int, int) {
	if shape.Sheet() {
		return box.Width, max(0, box.Height-1)
	}

	return max(1, box.Width-dialogFrameColumns), frame.BodyRows(box.Height)
}

// withDialog is the body of the screen — the rail and the detail — faint
// behind the open overlay drawn as a dialog in box over it, and over the
// dialog a last look goes back to, when it floats too.
func (m Model) withDialog(shape layout.Layout, box layout.Box) string {
	behind := m
	behind.overlay = nil
	layers := []*lipgloss.Layer{lipgloss.NewLayer(m.backdrop(behind.body(shape)))}

	if look, looks := m.overlay.(lastLook); looks {
		under := m
		under.overlay = look.back

		if underBox, floats := under.dialogBox(); floats {
			layers = append(layers, under.dialogLayer(shape, underBox, 1))
		}
	}

	return lipgloss.NewCompositor(append(layers, m.dialogLayer(shape, box, 2))...).Render() //nolint:mnd // above both
}

// dialogLayer is the open overlay drawn as a dialog in box, depth layers up.
func (m Model) dialogLayer(shape layout.Layout, box layout.Box, depth int) *lipgloss.Layer {
	width, rows := dialogInside(shape, box)
	title, content, style := m.overlayContent(width, rows)

	drawn := frame.Render(title, content, box.Width, box.Height, style)
	if shape.Sheet() {
		drawn = frame.Plain(title, content, box.Width, box.Height, style)
	}

	return lipgloss.NewLayer(drawn).X(box.X).Y(box.Y - shape.Detail.Y).Z(depth)
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
