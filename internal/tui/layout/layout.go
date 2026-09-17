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
	// the detail takes the whole body. Down to 80 columns the two sit side by
	// side; below that a 24-column rail beside a 50-column detail serves neither.
	collapseBelow = 80

	// borderlessBelow is the width under which even the detail's border goes:
	// at under 60 columns, two columns of box-drawing are text that does not
	// fit.
	borderlessBelow = 60

	// compactSpineBelow is the height under which the spine drops its labels.
	compactSpineBelow = 24

	// compactContent is the content rows a rail pane without focus keeps.
	// focusedMinimum is the least content a focused pane gets before sharing the
	// content rows evenly is the better use of a short terminal.
	compactContent = 2
	focusedMinimum = 4

	// railRules is the rules a shared rail spends: one around and one between
	// the panes, so N panes take N+1 rule rows rather than 2N borders.
	railRules = 1

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

// Compute lays out a terminal of the given size with railPanes stacked panels,
// the one at index focused having focus.
func Compute(width, height, railPanes, focused int) Layout {
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
	result.Rail = stack(railWidth, body, railPanes, focused)
	result.Detail = Box{X: railWidth, Y: spineRows, Width: width - railWidth, Height: body}

	return result
}

// noticeRows is the one row a notice takes above the footer.
const noticeRows = 1

// ComputeWithNotice is Compute with a row reserved for a notice above the
// footer: the body shrinks by that row so the rail and detail stay aligned, the
// notice takes the row the footer would have had, and the footer moves to the
// true bottom. It returns the layout and the notice's box.
func ComputeWithNotice(width, height, railPanes, focused int) (Layout, Box) {
	result := Compute(width, height-noticeRows, railPanes, focused)
	notice := Box{X: 0, Y: result.Footer.Y, Width: width, Height: noticeRows}
	result.Footer.Y = max(0, height-footerRows)

	return result, notice
}

// Collapsed reports whether the rail was dropped for lack of width.
func (l Layout) Collapsed() bool {
	return len(l.Rail) == 0
}

// Borderless reports a terminal too narrow to spend columns on a border.
func (l Layout) Borderless() bool {
	return l.Detail.Width < borderlessBelow
}

// CompactSpine reports a terminal too short for the spine's labels.
func (l Layout) CompactSpine() bool {
	return l.Footer.Y+footerRows < compactSpineBelow
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

// stack splits the rail's content rows between the panes — compact panes
// without focus and the rest to the focused one, or evenly on a terminal too
// short for that to help — and gives each pane a box that spans its content and
// the shared rule above it, so the boxes stay contiguous for hit-testing.
func stack(width, body, panes, focused int) []Box {
	content := allocateContent(max(0, body-panes-railRules), panes, focused)

	boxes := make([]Box, 0, panes)
	row := spineRows

	for _, rows := range content {
		height := rows + railRules
		boxes = append(boxes, Box{X: 0, Y: row, Width: width, Height: height})
		row += height
	}

	return boxes
}

// allocateContent splits the content rows: compact for the unfocused panes and
// the rest to the focused one, or evenly when there is too little to spare.
func allocateContent(total, panes, focused int) []int {
	rows := evenHeights(total, panes)

	if total >= compactContent*(panes-1)+focusedMinimum {
		for index := range rows {
			rows[index] = compactContent
		}

		rows[focused] = total - compactContent*(panes-1)
	}

	return rows
}

// evenHeights splits body evenly. Leftover rows go to the top panes one each
// rather than being dropped, because a row lost at the bottom of the screen
// shows up as a gap.
func evenHeights(body, panes int) []int {
	heights := make([]int, panes)
	base, leftover := body/panes, body%panes

	for index := range heights {
		heights[index] = base
		if index < leftover {
			heights[index]++
		}
	}

	return heights
}

// clamp keeps value within [low, high].
func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
