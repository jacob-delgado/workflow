// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"fmt"
	"testing"

	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// railPanes is the number of stacked panels the interface draws: Issues,
// Branch, Commits, Review, the messaging pane, Reviews and Tasks.
const railPanes = 7

// focusedFirst is a rail of panes with focus on the first.
func focusedFirst(panes int) layout.Rail {
	return layout.Rail{Panes: panes, Focused: 0}
}

func TestComputeReservesASpineAndAFooter(t *testing.T) {
	t.Parallel()

	// Act
	got := layout.Compute(layout.Terminal{Width: 120, Height: 40}, focusedFirst(railPanes))

	// Assert
	if got.Spine != (layout.Box{X: 0, Y: 0, Width: 120, Height: 1}) {
		t.Errorf("Spine = %+v, want the whole top row", got.Spine)
	}

	if got.Footer != (layout.Box{X: 0, Y: 39, Width: 120, Height: 1}) {
		t.Errorf("Footer = %+v, want the whole bottom row", got.Footer)
	}
}

func TestComputeWithNoticeReservesARowAboveTheFooter(t *testing.T) {
	t.Parallel()

	// Act
	shape, notice := layout.ComputeWithNotice(layout.Terminal{Width: 120, Height: 40}, focusedFirst(railPanes))

	// Assert
	// The notice sits just above the footer, the footer stays at the bottom, and
	// the body loses exactly the notice's row.
	plain := layout.Compute(layout.Terminal{Width: 120, Height: 40}, focusedFirst(railPanes))
	if notice.Y != 38 || shape.Footer.Y != 39 || shape.Detail.Height != plain.Detail.Height-1 {
		t.Errorf("ComputeWithNotice = notice %+v, footer %+v, detail %+v; want a row reserved above a bottom footer",
			notice, shape.Footer, shape.Detail)
	}
}

func TestEightyColumnsKeepsTheRailBesideTheDetail(t *testing.T) {
	t.Parallel()

	// Act
	got := layout.Compute(layout.Terminal{Width: 80, Height: 24}, focusedFirst(railPanes))

	// Assert
	if got.Collapsed() || len(got.Rail) != railPanes || got.Detail.Width != 56 {
		t.Errorf("Compute(80, 24) = collapsed %v, %d rail panes, detail width %d; want a rail beside a 56-column detail",
			got.Collapsed(), len(got.Rail), got.Detail.Width)
	}
}

func TestComputeGivesTheDetailWhatTheRailLeaves(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height int
		rails         int
		detail        layout.Box
	}{
		"beside a rail": {width: 120, height: 40, rails: railPanes, detail: layout.Box{X: 36, Y: 1, Width: 84, Height: 38}},
		// With no rail, the detail takes the whole body rather than shrinking
		// beside an empty column.
		"the whole body once the rail collapses": {
			width: 70, height: 30, rails: 0, detail: layout.Box{X: 0, Y: 1, Width: 70, Height: 28},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := layout.Compute(layout.Terminal{Width: tt.width, Height: tt.height}, focusedFirst(railPanes))

			// Assert
			if len(got.Rail) != tt.rails || got.Collapsed() != (tt.rails == 0) || got.Detail != tt.detail {
				t.Errorf("Compute(%d, %d) = %d rail panes (collapsed %v), detail %+v; want %d, %+v",
					tt.width, tt.height, len(got.Rail), got.Collapsed(), got.Detail, tt.rails, tt.detail)
			}
		})
	}
}

func TestComputeClampsTheRailWidth(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width int
		want  int
	}{
		// 30% of 120 is 36, inside the clamp.
		"a roomy terminal": {width: 120, want: 36},
		// 30% of 90 is 27, above the floor.
		"at the collapse threshold": {width: 90, want: 27},
		// 30% of 300 is 90, which would waste the screen on a list of titles.
		"a very wide terminal is capped": {width: 300, want: 40},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := layout.Compute(layout.Terminal{Width: tt.width, Height: 40}, focusedFirst(railPanes))

			// Assert
			if len(got.Rail) != railPanes || got.Rail[0].Width != tt.want {
				t.Errorf("rail at %d columns = %+v, want %d panes %d wide", tt.width, got.Rail, railPanes, tt.want)
			}
		})
	}
}

func TestTheFocusedPaneTakesTheRoomTheOthersDoNotNeed(t *testing.T) {
	t.Parallel()

	// 40 rows less the spine and footer leave 38 for the shared rail. Each pane's
	// box holds one shared-rule row above two rows of content when it is not
	// focused; the focused one gets the rest, so the list someone is working in
	// is the one that can be read. The boxes sum to one less than the body, the
	// spare row being the rail's bottom rule.
	cases := map[string]struct {
		height  int
		focused int
		want    []int
	}{
		"the first":  {height: 40, focused: 0, want: []int{19, 3, 3, 3, 3, 3, 3}},
		"the middle": {height: 40, focused: 2, want: []int{3, 3, 19, 3, 3, 3, 3}},
		"the last":   {height: 40, focused: 6, want: []int{3, 3, 3, 3, 3, 3, 19}},
		// 16 rows leave 14, and 6 content rows after the rules: too few to give
		// the focused pane a useful height after six compact ones, so the panes
		// share them, the top six a row each and the last none, rather than
		// drop a row at the bottom of the screen, where it is a visible gap.
		"a short terminal shares evenly": {height: 16, focused: 3, want: []int{2, 2, 2, 2, 2, 2, 1}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			rail := layout.Rail{Panes: railPanes, Focused: tt.focused}

			// Act & Assert
			requireStacked(t, layout.Compute(layout.Terminal{Width: 120, Height: tt.height}, rail), tt.want, tt.height-3)
		})
	}
}

func TestTheFocusedPaneGrowsFirstThenTheOthersThenItAgain(t *testing.T) {
	t.Parallel()

	// Seven panes spend eight rule rows, so 80x24 leaves 14 content rows: the
	// others keep one each and the focused pane takes eight. Rows past that go
	// to the others, top first, until each has two, and then to the focused one.
	cases := map[string]struct {
		height int
		want   []int
	}{
		"80x24 gives the others a row":    {height: 24, want: []int{9, 2, 2, 2, 2, 2, 2}},
		"80x26 grows the top two others":  {height: 26, want: []int{9, 3, 3, 2, 2, 2, 2}},
		"80x30 gives the others two each": {height: 30, want: []int{9, 3, 3, 3, 3, 3, 3}},
		"80x32 grows the focused one":     {height: 32, want: []int{11, 3, 3, 3, 3, 3, 3}},
		"80x15 splits what it has evenly": {height: 15, want: []int{2, 2, 2, 2, 2, 1, 1}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			terminal := layout.Terminal{Width: 80, Height: tt.height}

			// Act & Assert
			requireStacked(t, layout.Compute(terminal, focusedFirst(railPanes)), tt.want, tt.height-3)
		})
	}
}

// requireStacked checks each rail pane's height, that they use every body row,
// and that each starts where the previous one ended.
func requireStacked(t *testing.T, got layout.Layout, heights []int, body int) {
	t.Helper()

	if len(got.Rail) != len(heights) {
		t.Fatalf("got %d rail panes, want %d", len(got.Rail), len(heights))
	}

	total := 0

	for index, box := range got.Rail {
		total += box.Height

		if box.Height != heights[index] {
			t.Errorf("rail[%d].Height = %d, want %d", index, box.Height, heights[index])
		}
	}

	if total != body {
		t.Errorf("rail heights sum to %d, want every body row used (%d)", total, body)
	}

	for index := 1; index < len(got.Rail); index++ {
		previous := got.Rail[index-1]
		if got.Rail[index].Y != previous.Y+previous.Height {
			t.Errorf("rail[%d].Y = %d, want %d", index, got.Rail[index].Y, previous.Y+previous.Height)
		}
	}
}

func TestTheShapeFollowsTheTerminal(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height                       int
		collapsed, borderless, compactSpine bool
	}{
		"roomy":             {width: 120, height: 40},
		"narrow":            {width: 79, height: 40, collapsed: true},
		"very narrow":       {width: 59, height: 40, collapsed: true, borderless: true},
		"short":             {width: 120, height: 23, compactSpine: true},
		"at the thresholds": {width: 60, height: 24, collapsed: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := layout.Compute(layout.Terminal{Width: tt.width, Height: tt.height}, focusedFirst(railPanes))

			// Assert
			if got.Collapsed() != tt.collapsed || got.Borderless() != tt.borderless || got.CompactSpine() != tt.compactSpine {
				t.Errorf("collapsed, borderless, compact spine = %v, %v, %v; want %v, %v, %v",
					got.Collapsed(), got.Borderless(), got.CompactSpine(), tt.collapsed, tt.borderless, tt.compactSpine)
			}
		})
	}
}

func TestComputeSurvivesATinyTerminal(t *testing.T) {
	t.Parallel()

	// Act
	// A terminal can be resized to almost nothing mid-session. Negative sizes
	// would panic inside the renderer, so everything floors at zero.
	got := layout.Compute(layout.Terminal{Width: 120, Height: 1}, focusedFirst(railPanes))

	// Assert
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

	cases := map[string]struct {
		width, height int
		column, row   int
		want          int
		wantOK        bool
	}{
		"top of the first pane":  {width: 120, height: 40, column: 0, row: 1, want: 0, wantOK: true},
		"inside the third pane":  {width: 120, height: 40, column: 10, row: 24, want: 2, wantOK: true},
		"last row of the rail":   {width: 120, height: 40, column: 35, row: 37, want: 6, wantOK: true},
		"the spine is not rail":  {width: 120, height: 40, column: 5, row: 0, wantOK: false},
		"the detail is not rail": {width: 120, height: 40, column: 60, row: 10, wantOK: false},
		"the footer is not rail": {width: 120, height: 40, column: 5, row: 39, wantOK: false},
		"a collapsed layout":     {width: 70, height: 30, column: 5, row: 5, wantOK: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			shape := layout.Compute(layout.Terminal{Width: tt.width, Height: tt.height}, focusedFirst(railPanes))

			// Act
			index, ok := shape.RailAt(tt.column, tt.row)

			// Assert
			if ok != tt.wantOK || (ok && index != tt.want) {
				t.Errorf("RailAt(%d, %d) = %d, %v; want %d, %v", tt.column, tt.row, index, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestPanesPastSevenShrinkTheOthersToARowSoTheFocusedOneStaysUseful(t *testing.T) {
	t.Parallel()

	// At 80x24 an even split of eight or nine panes leaves the focused one a
	// row or two, too few to read; the others drop to one row, a line of what
	// each holds, and the focused one keeps at least four.
	cases := map[string]struct {
		panes int
		want  []int
	}{
		"eight panes": {panes: 8, want: []int{7, 2, 2, 2, 2, 2, 2, 2}},
		"nine panes":  {panes: 9, want: []int{5, 2, 2, 2, 2, 2, 2, 2, 2}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			requireStacked(t, layout.Compute(layout.Terminal{Width: 80, Height: 24}, focusedFirst(tt.panes)), tt.want, 21)
		})
	}
}

func TestATallerTerminalNeverGivesTheFocusedPaneFewerRows(t *testing.T) {
	t.Parallel()

	// Act
	drops := focusedDrops(10, 60, 7, 8, 9)

	// Assert
	for _, drop := range drops {
		t.Error(drop)
	}
}

// focusedDrops says each height, from shortest to tallest, at which a rail of
// each count of panes gives the focused pane fewer rows than a row shorter.
func focusedDrops(shortest, tallest int, counts ...int) []string {
	var drops []string

	for _, panes := range counts {
		previous := 0

		for height := shortest; height <= tallest; height++ {
			focused := layout.Compute(layout.Terminal{Width: 80, Height: height}, focusedFirst(panes)).Rail[0].Height
			if focused < previous {
				drops = append(drops, fmt.Sprintf("%d panes at %d rows give the focused one %d rows, fewer than %d",
					panes, height, focused, previous))
			}

			previous = focused
		}
	}

	return drops
}

func TestANoticeRowIsGivenOnlyWhereTheFocusedPaneKeepsItsRows(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		width, height int
		want          bool
	}{
		"at 80 by 24 the row would starve the focused pane": {width: 80, height: 24, want: false},
		"a taller terminal has the row to spare":            {width: 80, height: 30, want: true},
		"with the rail folded nothing is starved":           {width: 60, height: 24, want: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := layout.NoticeKeepsFocus(layout.Terminal{Width: tt.width, Height: tt.height}, focusedFirst(9))

			// Assert
			if got != tt.want {
				t.Errorf("NoticeKeepsFocus(%d, %d) = %v, want %v", tt.width, tt.height, got, tt.want)
			}
		})
	}
}
