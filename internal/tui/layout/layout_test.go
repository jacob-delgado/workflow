// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// railPanes is the number of stacked panels the design calls for: Issues,
// Branch, Commits, Review and Slack.
const railPanes = 5

func TestComputeReservesASpineAndAFooter(t *testing.T) {
	t.Parallel()

	got := layout.Compute(120, 40, railPanes)

	if got.Spine != (layout.Box{X: 0, Y: 0, Width: 120, Height: 1}) {
		t.Errorf("Spine = %+v, want the whole top row", got.Spine)
	}

	if got.Footer != (layout.Box{X: 0, Y: 39, Width: 120, Height: 1}) {
		t.Errorf("Footer = %+v, want the whole bottom row", got.Footer)
	}
}

func TestComputePlacesTheRailBesideTheDetail(t *testing.T) {
	t.Parallel()

	got := layout.Compute(120, 40, railPanes)

	if got.Collapsed() {
		t.Fatal("Collapsed() = true at 120 columns, want a rail")
	}

	if len(got.Rail) != railPanes {
		t.Fatalf("len(Rail) = %d, want %d", len(got.Rail), railPanes)
	}

	// 30% of 120 is 36, inside the clamp.
	railWidth := got.Rail[0].Width
	if railWidth != 36 {
		t.Errorf("rail width = %d, want 36", railWidth)
	}

	want := layout.Box{X: 36, Y: 1, Width: 84, Height: 38}
	if got.Detail != want {
		t.Errorf("Detail = %+v, want %+v", got.Detail, want)
	}
}

func TestComputeClampsTheRailWidth(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width int
		want  int
	}{
		// 30% of 90 is 27, above the floor.
		"at the collapse threshold": {width: 90, want: 27},
		// 30% of 300 is 90, which would waste the screen on a list of titles.
		"a very wide terminal is capped": {width: 300, want: 40},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := layout.Compute(tt.width, 40, railPanes)
			if got.Rail[0].Width != tt.want {
				t.Errorf("rail width at %d columns = %d, want %d", tt.width, got.Rail[0].Width, tt.want)
			}
		})
	}
}

func TestComputeGivesLeftoverRowsToTheTopPanes(t *testing.T) {
	t.Parallel()

	// 40 rows less the spine and footer leaves 38. Split five ways that is 7
	// each with 3 over, which go to the first three panes rather than being
	// dropped — a row lost at the bottom of the screen is a visible gap.
	got := layout.Compute(120, 40, railPanes)

	wantHeights := []int{8, 8, 8, 7, 7}
	total := 0

	for index, box := range got.Rail {
		total += box.Height

		if box.Height != wantHeights[index] {
			t.Errorf("rail[%d].Height = %d, want %d", index, box.Height, wantHeights[index])
		}
	}

	if total != 38 {
		t.Errorf("rail heights sum to %d, want every body row used (38)", total)
	}

	// Each pane starts where the previous one ended.
	for index := 1; index < len(got.Rail); index++ {
		previous := got.Rail[index-1]
		if got.Rail[index].Y != previous.Y+previous.Height {
			t.Errorf("rail[%d].Y = %d, want %d", index, got.Rail[index].Y, previous.Y+previous.Height)
		}
	}
}

func TestComputeCollapsesTheRailOnANarrowTerminal(t *testing.T) {
	t.Parallel()

	got := layout.Compute(80, 30, railPanes)

	if !got.Collapsed() {
		t.Fatal("Collapsed() = false at 80 columns, want the rail gone")
	}

	// With no rail, the detail takes the whole body rather than shrinking
	// beside an empty column.
	want := layout.Box{X: 0, Y: 1, Width: 80, Height: 28}
	if got.Detail != want {
		t.Errorf("Detail = %+v, want the whole body %+v", got.Detail, want)
	}
}

func TestComputeSurvivesATinyTerminal(t *testing.T) {
	t.Parallel()

	// A terminal can be resized to almost nothing mid-session. Negative sizes
	// would panic inside the renderer, so everything floors at zero.
	got := layout.Compute(120, 1, railPanes)

	if got.Detail.Height < 0 {
		t.Errorf("Detail.Height = %d, want it floored at zero", got.Detail.Height)
	}

	for index, box := range got.Rail {
		if box.Height < 0 {
			t.Errorf("rail[%d].Height = %d, want it floored at zero", index, box.Height)
		}
	}
}

func TestRailAt(t *testing.T) {
	t.Parallel()

	got := layout.Compute(120, 40, railPanes)

	cases := map[string]struct {
		column, row int
		want        int
		wantOK      bool
	}{
		"top of the first pane":  {column: 0, row: 1, want: 0, wantOK: true},
		"inside the third pane":  {column: 10, row: 20, want: 2, wantOK: true},
		"last row of the rail":   {column: 35, row: 38, want: 4, wantOK: true},
		"the spine is not rail":  {column: 5, row: 0, wantOK: false},
		"the detail is not rail": {column: 60, row: 10, wantOK: false},
		"the footer is not rail": {column: 5, row: 39, wantOK: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			index, ok := got.RailAt(tt.column, tt.row)
			if ok != tt.wantOK {
				t.Fatalf("RailAt(%d, %d) ok = %v, want %v", tt.column, tt.row, ok, tt.wantOK)
			}

			if ok && index != tt.want {
				t.Errorf("RailAt(%d, %d) = %d, want %d", tt.column, tt.row, index, tt.want)
			}
		})
	}
}

func TestRailAtOnACollapsedLayout(t *testing.T) {
	t.Parallel()

	if _, ok := layout.Compute(80, 30, railPanes).RailAt(5, 5); ok {
		t.Error("RailAt found a rail pane on a collapsed layout")
	}
}
