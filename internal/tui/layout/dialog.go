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
// footer, never wider than the screen less a margin each side nor taller than
// the body less two rows, and at least dialogLeast columns wide. Below 60
// columns there is no room to float, and it is a sheet over the whole body.
func (l Layout) Dialog(want Size) Box {
	body := Box{X: 0, Y: l.Detail.Y, Width: l.Footer.Width, Height: l.Detail.Height}
	if body.Width < BorderlessBelow {
		return body
	}

	width := clamp(want.Width, dialogLeast, body.Width-2*dialogMargin)
	height := clamp(want.Height, 1, body.Height-dialogMargin)

	return Box{
		X: (body.Width - width) / sides, Y: body.Y + (body.Height-height)/thirds,
		Width: width, Height: height,
	}
}
