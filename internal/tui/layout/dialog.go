// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package layout

// A dialog's bounds: never narrower than dialogLeast columns, and leaving
// dialogMargin columns and rows of the screen behind it showing.
const (
	dialogLeast  = 40
	dialogMargin = 2
	// thirds puts a dialog a third of the spare rows down, in the body's
	// upper third, where the eye starts.
	thirds = 3
	// sides is the two sides a dialog's spare columns are shared between.
	sides = 2
)

// Size is how big something would like to be, in cells.
type Size struct {
	Width, Height int
}

// Dialog is where a dialog that would like to be want goes: centered across
// the screen and in the upper third of the body between the spine and the
// footer, within DialogRoom, and at least dialogLeast columns wide. Below 60
// columns there is no room to float, and it is a sheet over the whole body.
func (l Layout) Dialog(want Size) Box {
	body := Box{X: 0, Y: l.Detail.Y, Width: l.Footer.Width, Height: l.Detail.Height}
	if l.Sheet() {
		return body
	}

	room := l.DialogRoom()
	width := clamp(want.Width, dialogLeast, room.Width)
	height := clamp(want.Height, 1, room.Height)

	return Box{
		X: (body.Width - width) / sides, Y: body.Y + (body.Height-height)/thirds,
		Width: width, Height: height,
	}
}

// Sheet reports a screen too narrow to float a dialog over, below 60
// columns: a dialog there is a sheet over the whole body, unframed as the
// detail pane is, however narrow what it holds.
func (l Layout) Sheet() bool {
	return l.Footer.Width < BorderlessBelow
}

// DialogRoom is the most a floating dialog may take, its frame included: the
// screen less a margin each side, and the body less two rows. A dialog sized
// to fit what it says measures it within this room.
func (l Layout) DialogRoom() Size {
	return Size{Width: l.Footer.Width - sides*dialogMargin, Height: l.Detail.Height - dialogMargin}
}
