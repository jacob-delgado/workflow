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
