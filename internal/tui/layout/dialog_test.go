// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// screenOf is the layout of a terminal of the given size.
func screenOf(width, height int) layout.Layout {
	return layout.Compute(layout.Terminal{Width: width, Height: height}, focusedFirst(railPanes))
}

func TestADialogIsCenteredInTheUpperThirdOfTheBody(t *testing.T) {
	t.Parallel()

	// Act
	got := screenOf(80, 24).Dialog(layout.Size{Width: 64, Height: 18})

	// Assert
	if got != (layout.Box{X: 8, Y: 2, Width: 64, Height: 18}) {
		t.Errorf("Dialog = %+v, want 64 by 18, centered, a third of the spare rows above it", got)
	}
}

func TestADialogBelowSixtyColumnsFillsTheBody(t *testing.T) {
	t.Parallel()

	// Arrange
	shape := screenOf(59, 24)

	// Act
	got := shape.Dialog(layout.Size{Width: 40, Height: 6})

	// Assert
	if got != shape.Detail {
		t.Errorf("Dialog = %+v, want the whole body %+v", got, shape.Detail)
	}
}

func TestADialogNeverOutgrowsTheBody(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		want layout.Size
		box  layout.Box
	}{
		"too wide and too tall": {
			want: layout.Size{Width: 500, Height: 500}, box: layout.Box{X: 2, Y: 1, Width: 116, Height: 36},
		},
		"narrower than the least": {
			want: layout.Size{Width: 10, Height: 4}, box: layout.Box{X: 40, Y: 12, Width: 40, Height: 4},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := screenOf(120, 40).Dialog(tt.want)

			// Assert
			if got != tt.box {
				t.Errorf("Dialog = %+v, want %+v", got, tt.box)
			}
		})
	}
}

func TestADialogWantingAllItsRoomGetsIt(t *testing.T) {
	t.Parallel()

	cases := map[string]layout.Terminal{
		"at 60 columns":  {Width: 60, Height: 24},
		"at 80 columns":  {Width: 80, Height: 24},
		"at 120 columns": {Width: 120, Height: 40},
	}

	for name, terminal := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			shape := screenOf(terminal.Width, terminal.Height)
			room := shape.DialogRoom()

			// Act
			got := shape.Dialog(room)

			// Assert
			if (layout.Size{Width: got.Width, Height: got.Height}) != room {
				t.Errorf("Dialog(%+v) = %+v, want a dialog the size of its room", room, got)
			}
		})
	}
}

func TestADialogIsASheetOnlyBelowSixtyColumns(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width int
		sheet bool
	}{
		"at 59 columns":  {width: 59, sheet: true},
		"at 60 columns":  {width: 60, sheet: false},
		"at 120 columns": {width: 120, sheet: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := screenOf(tt.width, 24).Sheet(); got != tt.sheet {
				t.Errorf("Sheet at %d columns = %t, want %t", tt.width, got, tt.sheet)
			}
		})
	}
}
