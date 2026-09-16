// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package layout decides where each region of the terminal interface goes for
// a given terminal size.
//
// It is pure arithmetic, kept apart from the Bubble Tea model so that the parts
// most likely to be wrong at the edges — clamping, leftover rows, a terminal
// resized to almost nothing — can be tested directly rather than inferred from
// a rendered screen.
package layout

// Fixed regions and the rail's sizing rules.
const (
	spineRows  = 1
	footerRows = 1

	// railPercent of the width goes to the rail, clamped to [railMin, railMax]:
	// narrow enough to leave the detail readable, wide enough to hold a title.
	railPercent = 30
	railMin     = 24
	railMax     = 40

	// collapseBelow is the width under which the rail is dropped entirely and
	// the detail takes the whole body. A 24-column rail beside a 50-column
	// detail serves neither.
	collapseBelow = 90

	percent = 100
)

// Box is a rectangle on screen, in cells.
type Box struct {
	X, Y          int
	Width, Height int
}

// Contains reports whether a cell falls inside the box.
func (b Box) Contains(column, row int) bool {
	return column >= b.X && column < b.X+b.Width && row >= b.Y && row < b.Y+b.Height
}

// Layout is where every region goes.
type Layout struct {
	Spine  Box
	Rail   []Box
	Detail Box
	Footer Box
}

// Compute lays out a terminal of the given size with railPanes stacked panels.
func Compute(width, height, railPanes int) Layout {
	body := max(0, height-spineRows-footerRows)

	result := Layout{
		Spine:  Box{X: 0, Y: 0, Width: width, Height: spineRows},
		Rail:   nil,
		Detail: Box{X: 0, Y: spineRows, Width: width, Height: body},
		Footer: Box{X: 0, Y: max(0, height-footerRows), Width: width, Height: footerRows},
	}

	if width < collapseBelow {
		return result
	}

	railWidth := clamp(width*railPercent/percent, railMin, railMax)
	result.Rail = stack(railWidth, body, railPanes)
	result.Detail = Box{X: railWidth, Y: spineRows, Width: width - railWidth, Height: body}

	return result
}

// Collapsed reports whether the rail was dropped for lack of width.
func (l Layout) Collapsed() bool {
	return len(l.Rail) == 0
}

// RailAt reports which rail pane a cell falls in.
func (l Layout) RailAt(column, row int) (int, bool) {
	for index, box := range l.Rail {
		if box.Contains(column, row) {
			return index, true
		}
	}

	return 0, false
}

// stack splits the body height between the rail panes. Leftover rows go to the
// top panes one each rather than being dropped, because a row lost at the
// bottom of the screen shows up as a gap.
func stack(width, body, panes int) []Box {
	boxes := make([]Box, 0, panes)
	base, leftover := body/panes, body%panes
	row := spineRows

	for index := range panes {
		height := base
		if index < leftover {
			height++
		}

		boxes = append(boxes, Box{X: 0, Y: row, Width: width, Height: height})
		row += height
	}

	return boxes
}

// clamp keeps value within [low, high].
func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
